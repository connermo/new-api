# New-API Kubernetes 部署指南

本目录包含在 Kubernetes 集群上部署 New-API 服务的配置文件（1主2从架构）。

## 架构说明

部署架构包括：
- **1个主节点** (new-api-master): 处理读写操作
- **2个从节点** (new-api-slave): 处理只读操作和负载均衡
- **MySQL**: 共享数据库
- **Redis**: 共享缓存和会话存储
- **持久化存储**: 数据和日志持久化

## 前置要求

1. 本地 Kubernetes 集群已运行（如 minikube, kind, Docker Desktop Kubernetes 等）
2. `kubectl` 命令行工具已安装并配置
3. 集群支持 PersistentVolume（如果使用 minikube，需要启用默认的 hostPath provisioner）

## 快速部署

### 方式一：使用部署脚本（推荐）

```bash
cd k8s
chmod +x deploy.sh
./deploy.sh
```

### 方式二：手动部署

```bash
cd k8s

# 1. 创建命名空间
kubectl apply -f namespace.yaml

# 2. 部署 MySQL
kubectl apply -f mysql.yaml

# 3. 部署 Redis
kubectl apply -f redis.yaml

# 4. 等待 MySQL 和 Redis 就绪
kubectl wait --for=condition=ready pod -l app=mysql -n new-api --timeout=300s
kubectl wait --for=condition=ready pod -l app=redis -n new-api --timeout=300s

# 5. 部署 New-API
kubectl apply -f new-api.yaml

# 6. 等待 New-API 就绪
kubectl wait --for=condition=ready pod -l app=new-api -n new-api --timeout=300s
```

## 访问服务

部署完成后，可以通过以下方式访问：

- **本地访问**: http://localhost:30000
- **集群内访问**: http://new-api.new-api.svc.cluster.local:3000

## 配置说明

### 关键配置

1. **Session Secret** (k8s/new-api.yaml)
   ```yaml
   SESSION_SECRET: "your-random-session-secret-change-this-123456"
   ```
   ⚠️ **生产环境必须修改为随机字符串！**

2. **MySQL 密码** (k8s/mysql.yaml)
   ```yaml
   MYSQL_ROOT_PASSWORD: "123456"
   ```
   ⚠️ **生产环境必须修改为强密码！**

3. **存储大小**
   - MySQL: 10Gi (可在 mysql.yaml 中调整)
   - New-API 数据: 5Gi (可在 new-api.yaml 中调整)
   - New-API 日志: 5Gi (可在 new-api.yaml 中调整)

### 主从节点配置

- **主节点**: 1个副本，不设置 `NODE_TYPE` 环境变量
- **从节点**: 2个副本，设置 `NODE_TYPE=slave` 和 `SYNC_FREQUENCY=60`

Service 会自动负载均衡所有节点的流量。

## 管理命令

### 查看状态

```bash
# 查看所有资源
kubectl get all -n new-api

# 查看 Pod 状态
kubectl get pods -n new-api

# 查看服务
kubectl get svc -n new-api

# 查看持久化卷
kubectl get pvc -n new-api
```

### 查看日志

```bash
# 查看主节点日志
kubectl logs -f -n new-api -l role=master

# 查看从节点日志
kubectl logs -f -n new-api -l role=slave

# 查看特定 Pod 日志
kubectl logs -f -n new-api <pod-name>

# 查看 MySQL 日志
kubectl logs -f -n new-api -l app=mysql

# 查看 Redis 日志
kubectl logs -f -n new-api -l app=redis
```

### 扩缩容

```bash
# 增加从节点数量
kubectl scale deployment new-api-slave -n new-api --replicas=3

# 减少从节点数量
kubectl scale deployment new-api-slave -n new-api --replicas=1
```

### 更新镜像

```bash
# 更新主节点镜像
kubectl set image deployment/new-api-master -n new-api \
  new-api=ghcr.io/connermo/new-api:new-tag

# 更新从节点镜像
kubectl set image deployment/new-api-slave -n new-api \
  new-api=ghcr.io/connermo/new-api:new-tag
```

### 进入容器调试

```bash
# 进入 New-API 容器
kubectl exec -it -n new-api <pod-name> -- sh

# 进入 MySQL 容器
kubectl exec -it -n new-api <mysql-pod-name> -- mysql -uroot -p123456 new-api
```

### 端口转发（临时访问）

```bash
# 转发 New-API 服务到本地 8080 端口
kubectl port-forward -n new-api svc/new-api 8080:3000

# 转发 MySQL 到本地 3306 端口
kubectl port-forward -n new-api svc/mysql 3306:3306
```

## 卸载

### 完全删除（包括数据）

```bash
# 删除整个命名空间（会删除所有资源和数据）
kubectl delete namespace new-api
```

### 仅删除应用（保留数据）

```bash
# 删除应用
kubectl delete -f new-api.yaml
kubectl delete -f redis.yaml
kubectl delete -f mysql.yaml

# 数据卷会保留，重新部署后数据依然存在
```

## 故障排查

### Pod 无法启动

```bash
# 查看 Pod 事件
kubectl describe pod -n new-api <pod-name>

# 查看日志
kubectl logs -n new-api <pod-name>
```

### 存储问题

```bash
# 查看 PVC 状态
kubectl get pvc -n new-api

# 查看 PV 状态
kubectl get pv

# 如果使用 minikube，确保启用了默认存储类
minikube addons enable default-storageclass
minikube addons enable storage-provisioner
```

### MySQL 连接问题

```bash
# 检查 MySQL 是否就绪
kubectl get pods -n new-api -l app=mysql

# 查看 MySQL 日志
kubectl logs -n new-api -l app=mysql

# 测试 MySQL 连接
kubectl run -it --rm mysql-client --image=mysql:8.2 -n new-api -- \
  mysql -h mysql -uroot -p123456 new-api
```

### Redis 连接问题

```bash
# 检查 Redis 是否就绪
kubectl get pods -n new-api -l app=redis

# 测试 Redis 连接
kubectl run -it --rm redis-client --image=redis:latest -n new-api -- \
  redis-cli -h redis ping
```

## 性能优化建议

1. **资源限制**: 为生产环境添加资源请求和限制
   ```yaml
   resources:
     requests:
       memory: "256Mi"
       cpu: "250m"
     limits:
       memory: "512Mi"
       cpu: "500m"
   ```

2. **HPA 自动扩缩容**: 根据 CPU/内存使用率自动扩缩从节点
   ```bash
   kubectl autoscale deployment new-api-slave -n new-api \
     --min=2 --max=5 --cpu-percent=80
   ```

3. **使用 Ingress**: 对于生产环境，建议使用 Ingress 控制器而不是 NodePort

4. **持久化存储优化**: 根据实际需求选择合适的 StorageClass

## 监控

建议集成以下监控方案：
- Prometheus + Grafana: 监控资源使用情况
- EFK (Elasticsearch + Fluentd + Kibana): 集中日志管理

## 安全建议

1. ✅ 修改默认密码（SESSION_SECRET, MYSQL_ROOT_PASSWORD）
2. ✅ 使用 Kubernetes Secrets 存储敏感信息
3. ✅ 启用 RBAC 限制访问权限
4. ✅ 使用 NetworkPolicy 限制网络访问
5. ✅ 定期更新镜像版本
6. ✅ 启用 TLS/SSL 加密通信

## 更多信息

- [New-API 项目主页](https://github.com/QuantumNous/new-api)
- [Kubernetes 官方文档](https://kubernetes.io/docs/)
