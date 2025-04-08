# **Implementation of a Multi-Node Load-Balancing Scheduler**

This is a custom Kubernetes scheduler implementation based on resource utilization. The scheduler uses Prometheus monitoring data to obtain node CPU and memory usage, and schedules Pods to the node with the lowest load.

Test usage The test uses 12 pod examples to verify the scheduler effect:

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

## Features

- Schedule Pods based on actual node resource utilization
- Comprehensive evaluation of CPU and memory metrics
- Fallback to simple round-robin scheduling when metrics are unavailable
- Complete RBAC permissions configuration
- Query node metrics via Prometheus
- Visual monitoring with Grafana
- Detailed scheduling logs

## Project Structure

```
manifests/
  ├── kind-config.yaml        # Kind cluster configuration
  ├── rbac.yaml              # RBAC permissions configuration
  ├── scheduler-deployment.yaml  # Scheduler deployment configuration
  ├── test-pods.yaml         # Test Pod configurations
  ├── node-monitor.yaml      # Prometheus node monitoring configuration
  ├── grafana-service.yaml   # Grafana service configuration
  └── grafana-dashboard.yaml # Grafana dashboard configuration
scheduler/
  ├── Dockerfile             # Docker image build configuration
  ├── go.mod                # Go module dependencies
  ├── go.sum                # Go module checksums
  └── main.go               # Scheduler main program
```

## Environment Requirements

- Kubernetes 1.21+
- Golang 1.21+
- Docker Desktop
- Kind
- Helm 3+
- Windows 10/11 or Linux

## Detailed Deployment Steps

### 1. Environment Preparation

```powershell
# Check Docker service status
Get-Service -Name com.docker.service

# Start the service if not running
Start-Service com.docker.service

# Verify Docker status
docker version
```

### 2. Create Cluster

```powershell
# Create Kind cluster
kind create cluster --config manifests/kind-config.yaml

# Verify cluster status
kubectl cluster-info
kubectl get nodes
```

### 3. Install Prometheus Stack

```powershell
# Create monitoring namespace
kubectl create namespace monitoring

# Add Helm repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install Prometheus Stack, If you fail, try again
helm install prometheus prometheus-community/kube-prometheus-stack `
  --namespace monitoring `
  --create-namespace `
  --wait
```

### 4. Configure Monitoring

```powershell
# Deploy node monitoring configuration
kubectl apply -f manifests/node-monitor.yaml

# Deploy Grafana dashboard configuration
kubectl apply -f manifests/grafana-dashboard.yaml

# Deploy Grafana service
kubectl apply -f manifests/grafana-service.yaml
```

### 5. Build and Deploy Scheduler

```powershell
# Build scheduler image
cd scheduler
docker build -t custom-scheduler:latest .

# Load image into the cluster
kind load docker-image custom-scheduler:latest

# Configure RBAC
kubectl apply -f ../manifests/rbac.yaml

# Deploy scheduler
kubectl apply -f ../manifests/scheduler-deployment.yaml
```

## Monitoring and Validation

### 1. Prometheus Validation

1. Check component status:
```powershell
# Check Prometheus components
kubectl get pods -n monitoring | findstr prometheus

# Verify ServiceMonitor configuration
kubectl get servicemonitor -n monitoring
```

2. Validate metric collection:
```powershell
# Port-forward Prometheus service
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
```

Visit http://localhost:9090 and validate the following queries:
```
# CPU usage query
100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)

# Memory usage query
100 * (1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes))
```

### 2. Grafana Monitoring

1. Access Grafana:
```powershell
# Port-forward Grafana service
kubectl port-forward -n monitoring svc/grafana-nodeport 3000:80
```

2. Login details:
- URL: http://localhost:3000
- Username: admin
- Password: prom-operator

3. Dashboard features:
- Node CPU usage trends
- Node memory usage trends
- Pod count per node
- Scheduler performance metrics

### 3. Test Scheduling Behavior

```powershell
# Deploy test Pods
kubectl create namespace k8s
kubectl apply -f manifests/test-pods.yaml

# View Pod distribution
kubectl get pods -n k8s -o wide

# Monitor resource usage
kubectl top nodes
kubectl top pods -n k8s
```

## Troubleshooting

### 1. Common Issues

- Pod stuck in Pending state:
```powershell
kubectl describe pod <pod-name> -n k8s
```

- Metric collection failure:
```powershell
# Check node-exporter
kubectl logs -n monitoring -l app=prometheus-node-exporter

# Validate metric endpoints
kubectl port-forward -n monitoring svc/prometheus-prometheus-node-exporter 9100:9100
curl http://localhost:9100/metrics
```

- Grafana access issues:
```powershell
# Inspect Grafana Pod
kubectl describe pod -n monitoring -l app.kubernetes.io/name=grafana
```

### 2. Connectivity Issues

```powershell
# Restart Docker service
Restart-Service com.docker.service

# Re-export kubeconfig
kind export kubeconfig

# Reload scheduler image
kind load docker-image custom-scheduler:latest
```

## Best Practices

1. Monitoring Recommendations
   - Regularly check scheduler logs
   - Monitor node load via Grafana dashboards
   - Set appropriate alert thresholds

2. Performance Optimization
   - Adjust metric collection intervals
   - Optimize scheduler algorithm weights
   - Set reasonable resource limits

3. Operational Advice
   - Regularly back up monitoring data
   - Keep Prometheus and Grafana updated
   - Periodically validate scheduling behavior

## Notes

1. Prerequisites
   - Ensure Docker Desktop is running properly
   - Verify Kubernetes cluster availability
   - Confirm Helm is functional

2. Monitoring Configuration
   - Ensure Prometheus is deployed correctly
   - Verify node-exporter runs on all nodes
   - Validate ServiceMonitor configurations

3. Security Recommendations
   - Regularly update Grafana passwords
   - Enforce RBAC permissions
   - Avoid exposing monitoring endpoints externally

4. Additional Notes
   - Thoroughly test in non-production environments
   - Regularly back up configurations
   - Check version compatibility
   [file content end]
