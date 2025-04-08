# **一个多节点负载均衡的调度器的实现**

这是一个基于资源使用率的自定义 Kubernetes 调度器实现。该调度器通过 Prometheus 监控数据来获取节点的 CPU 和内存使用情况，并将 Pod 调度到负载最低的节点上。

测试使用测试使用12个pod示例来验证调度器效果：

| Pod Name | CPU Requests (m) | Memory Requests (Mi) |
| -------- | ---------------- | -------------------- |
| pod-1    | 500              | 500                  |
| pod-2    | 1000             | 1024                 |
| pod-3    | 1000             | 1536                 |
| pod-4    | 2000             | 2048                 |
| pod-5    | 250              | 256                  |
| pod-6    | 1500             | 2048                 |
| pod-7    | 500              | 1024                 |
| pod-8    | 2000             | 4096                 |
| pod-9    | 3000             | 3072                 |
| pod-10   | 750              | 512                  |
| pod-11   | 1000             | 1024                 |
| pod-12   | 250              | 128                  |

## 功能特点

- 基于节点实际资源使用率进行调度
- 支持 CPU 和内存指标的综合评估 
- 当无法获取指标时自动回退到简单轮询调度
- 完整的 RBAC 权限配置
- 支持通过 Prometheus 查询节点指标
- 支持 Grafana 可视化监控
- 提供详细的调度日志

## 项目结构

```
manifests/
  ├── kind-config.yaml        # Kind 集群配置
  ├── rbac.yaml              # RBAC 权限配置
  ├── scheduler-deployment.yaml  # 调度器部署配置
  ├── test-pods.yaml         # 测试 Pod 配置
  ├── node-monitor.yaml      # Prometheus 节点监控配置
  ├── grafana-service.yaml   # Grafana 服务配置
  └── grafana-dashboard.yaml # Grafana 仪表板配置
scheduler/
  ├── Dockerfile             # 构建容器镜像配置
  ├── go.mod                # Go 模块依赖
  ├── go.sum                # Go 模块校验和
  └── main.go               # 调度器主程序
```

## 环境要求

- Kubernetes 1.21+
- Golang 1.21+
- Docker Desktop
- Kind
- Helm 3+
- Windows 10/11 或 Linux

## 详细部署步骤

### 1. 环境准备

```powershell
# 检查 Docker 服务状态
Get-Service -Name com.docker.service

# 如果服务未运行，启动服务
Start-Service com.docker.service

# 验证 Docker 运行状态
docker version
```

### 2. 创建集群

```powershell
# 创建 Kind 集群
kind create cluster --config manifests/kind-config.yaml

# 验证集群状态
kubectl cluster-info
kubectl get nodes
```

### 3. 安装 Prometheus Stack

```powershell
# 创建监控命名空间
kubectl create namespace monitoring

# 添加 Helm 仓库
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# 安装 Prometheus Stack，如果失败请多试几次
helm install prometheus prometheus-community/kube-prometheus-stack `
  --namespace monitoring `
  --create-namespace `
  --wait
```

### 4. 配置监控

```powershell
# 部署节点监控配置
kubectl apply -f manifests/node-monitor.yaml

# 部署 Grafana 仪表板配置
kubectl apply -f manifests/grafana-dashboard.yaml

# 部署 Grafana 服务
kubectl apply -f manifests/grafana-service.yaml
```

### 5. 构建和部署调度器

```powershell
# 构建调度器镜像
cd scheduler
docker build -t custom-scheduler:latest .

# 加载镜像到集群
kind load docker-image custom-scheduler:latest

# 配置 RBAC
kubectl apply -f ../manifests/rbac.yaml

# 部署调度器
kubectl apply -f ../manifests/scheduler-deployment.yaml
```

## 监控与验证

### 1. Prometheus 验证

1. 检查组件状态：
```powershell
# 检查 Prometheus 组件
kubectl get pods -n monitoring | findstr prometheus

# 验证 ServiceMonitor 配置
kubectl get servicemonitor -n monitoring
```

2. 验证指标采集：
```powershell
# 端口转发 Prometheus 服务
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
```

访问 http://localhost:9090 验证以下查询：
```
# CPU 使用率查询
100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)

# 内存使用率查询
100 * (1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes))
```

### 2. Grafana 监控

1. 访问 Grafana：
```powershell
# 端口转发 Grafana 服务
kubectl port-forward -n monitoring svc/grafana-nodeport 3000:80
```

2. 登录信息：
- URL: http://localhost:3000
- 用户名: admin
- 密码: prom-operator

3. 仪表板功能：
- 节点 CPU 使用率趋势图
- 节点内存使用率趋势图
- 每个节点的 Pod 数量统计
- 调度器性能指标

### 3. 测试调度效果

```powershell
# 部署测试 Pod
kubectl create namespace k8s
kubectl apply -f manifests/test-pods.yaml

# 查看 Pod 分布
kubectl get pods -n k8s -o wide

# 监控资源使用情况
kubectl top nodes
kubectl top pods -n k8s
```

## 故障排查

### 1. 常见问题处理

- Pod 一直处于 Pending：
```powershell
kubectl describe pod <pod-name> -n k8s
```

- 指标获取失败：
```powershell
# 检查 node-exporter
kubectl logs -n monitoring -l app=prometheus-node-exporter

# 验证指标端点
kubectl port-forward -n monitoring svc/prometheus-prometheus-node-exporter 9100:9100
curl http://localhost:9100/metrics
```

- Grafana 访问问题：
```powershell
# 检查 Grafana Pod
kubectl describe pod -n monitoring -l app.kubernetes.io/name=grafana
```

### 2. 连接问题处理

```powershell
# 重置 Docker 服务
Restart-Service com.docker.service

# 重新导出 kubeconfig
kind export kubeconfig

# 重新加载镜像
kind load docker-image custom-scheduler:latest
```

## 最佳实践

1. 监控建议
   - 定期检查调度器日志
   - 通过 Grafana 仪表板监控节点负载
   - 设置适当的告警阈值

2. 性能优化
   - 调整指标采集间隔
   - 优化调度算法权重
   - 设置合理的资源限制

3. 运维建议
   - 定期备份监控数据
   - 保持 Prometheus 和 Grafana 版本更新
   - 定期验证调度效果

## 注意事项

1. 部署前提
   - 确保 Docker Desktop 正常运行
   - 验证 Kubernetes 集群可用性
   - 检查 Helm 可用性

2. 监控配置
   - 确保 Prometheus 正确部署
   - 验证 node-exporter 在所有节点运行
   - 检查 ServiceMonitor 配置正确

3. 安全建议
   - 定期更新 Grafana 密码
   - 注意 RBAC 权限控制
   - 避免将监控端点暴露到外部

4. 其他注意事项
   - 测试环境充分验证
   - 定期备份配置
   - 关注版本兼容性