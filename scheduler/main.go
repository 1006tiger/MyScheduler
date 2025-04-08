package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type NodeMetrics struct {
	NodeName    string
	CPUUsage    float64
	MemoryUsage float64
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	for {
		pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
			FieldSelector: "spec.schedulerName=custom-scheduler,spec.nodeName=",
		})
		if err != nil {
			log.Printf("Error listing pods: %v", err)
			continue
		}

		for _, pod := range pods.Items {
			go schedulePod(clientset, &pod)
		}

		time.Sleep(1 * time.Second)
	}
}

func schedulePod(clientset *kubernetes.Clientset, pod *v1.Pod) {
	// 获取节点列表
	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error listing nodes: %v", err)
		return
	}

	// 初始化节点指标切片
	nodeMetrics := make([]NodeMetrics, 0, len(nodeList.Items))

	// 获取每个节点的指标
	for _, node := range nodeList.Items {
		metrics := getNodeMetricsForNode(&node)
		if metrics != nil {
			nodeMetrics = append(nodeMetrics, *metrics)
		}
	}

	var selectedNode string
	if len(nodeMetrics) == 0 {
		// 如果无法获取指标，使用简单的轮询调度
		log.Printf("No metrics available, falling back to simple round-robin scheduling")
		for _, node := range nodeList.Items {
			if !node.Spec.Unschedulable {
				selectedNode = node.Name
				break
			}
		}
		if selectedNode == "" {
			log.Printf("No available nodes for scheduling")
			return
		}
	} else {
		// 使用指标进行调度
		sort.Slice(nodeMetrics, func(i, j int) bool {
			return nodeMetrics[i].CPUUsage+nodeMetrics[i].MemoryUsage <
				nodeMetrics[j].CPUUsage+nodeMetrics[j].MemoryUsage
		})
		selectedNode = nodeMetrics[0].NodeName
	}

	log.Printf("Selected node %s for pod %s/%s", selectedNode, pod.Namespace, pod.Name)

	// 绑定 Pod 到选中的节点
	binding := &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		Target: v1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       selectedNode,
		},
	}

	err = clientset.CoreV1().Pods(pod.Namespace).Bind(context.TODO(), binding, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Error binding pod %s/%s to node %s: %v",
			pod.Namespace, pod.Name, selectedNode, err)
		return
	}
	log.Printf("Successfully bound pod %s/%s to node %s",
		pod.Namespace, pod.Name, selectedNode)
}

func getNodeMetricsForNode(node *v1.Node) *NodeMetrics {
	// 使用正确的 Prometheus 服务地址
	prometheusURL := "http://prometheus-operated.monitoring.svc:9090"

	// 修改 CPU 查询，使用 instance 标签
	cpuQuery := fmt.Sprintf(`100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle",instance="%s:9100"}[5m])) * 100)`, node.Name)

	log.Printf("Querying Prometheus at %s with query: %s", prometheusURL, cpuQuery)

	cpuResp, err := http.Get(fmt.Sprintf("%s/api/v1/query?query=%s", prometheusURL, cpuQuery))
	if err != nil {
		log.Printf("Error getting CPU metrics for node %s: %v", node.Name, err)
		return nil
	}
	defer cpuResp.Body.Close()

	// 修改响应结构以匹配 Prometheus API 格式
	var cpuResult struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric struct {
					Instance string `json:"instance"`
				} `json:"metric"`
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(cpuResp.Body).Decode(&cpuResult); err != nil {
		log.Printf("Error decoding CPU metrics: %v", err)
		return nil
	}

	if len(cpuResult.Data.Result) == 0 {
		log.Printf("No CPU metrics found for node %s", node.Name)
		return nil
	}

	// 修改内存查询，使用 instance 标签
	memQuery := fmt.Sprintf(`100 * (1 - (sum(node_memory_MemAvailable_bytes{instance="%s:9100"}) / sum(node_memory_MemTotal_bytes{instance="%s:9100"})))`,
		node.Name, node.Name)

	log.Printf("Memory Query: %s", memQuery)

	memResp, err := http.Get(fmt.Sprintf("%s/api/v1/query?query=%s", prometheusURL, memQuery))
	if err != nil {
		log.Printf("Error getting memory metrics: %v", err)
		return nil
	}
	defer memResp.Body.Close()

	var memResult struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(memResp.Body).Decode(&memResult); err != nil {
		log.Printf("Error decoding memory metrics: %v", err)
		return nil
	}

	if len(memResult.Data.Result) == 0 {
		log.Printf("No memory metrics found for node %s", node.Name)
		return nil
	}

	metrics := &NodeMetrics{
		NodeName:    node.Name,
		CPUUsage:    getFloat64Value(cpuResult.Data.Result[0].Value[1]),
		MemoryUsage: getFloat64Value(memResult.Data.Result[0].Value[1]),
	}

	log.Printf("Node %s metrics - CPU: %.2f%%, Memory: %.2f%%",
		node.Name, metrics.CPUUsage, metrics.MemoryUsage)

	return metrics
}

func getFloat64Value(v interface{}) float64 {
	switch v := v.(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}
