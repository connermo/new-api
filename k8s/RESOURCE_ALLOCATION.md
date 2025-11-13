# New-API K8s 资源分配说明

## 当前部署配置

### 架构概览
- **1个主节点** (new-api-master)
- **3个从节点** (new-api-slave)
- **1个 MySQL 数据库**
- **1个 Redis 缓存**

### 预期性能
- **预估 QPS**: ~3000-4000
  - 主节点: ~1000 QPS
  - 从节点: ~3000 QPS (每个 ~1000 QPS)

## 详细资源配置

### New-API 主节点 (Master)

```yaml
replicas: 1
resources:
  requests:
    cpu: 1000m       # 1 核心
    memory: 1Gi      # 1GB
  limits:
    cpu: 3000m       # 3 核心，处理突发流量
    memory: 4Gi      # 4GB
```

**用途**: 处理写操作和部分读操作

### New-API 从节点 (Slave)

```yaml
replicas: 3
resources:
  requests:
    cpu: 1000m       # 每个 1 核心
    memory: 1Gi      # 每个 1GB
  limits:
    cpu: 2000m       # 每个 2 核心，处理突发流量
    memory: 2Gi      # 每个 2GB
```

**用途**: 处理只读操作，负载均衡
**环境变量**:
- `NODE_TYPE=slave`
- `SYNC_FREQUENCY=60` (秒)

### MySQL 数据库

```yaml
replicas: 1
resources:
  requests:
    cpu: 2000m       # 2 核心，支持高并发数据库查询
    memory: 4Gi      # 4GB
  limits:
    cpu: 4000m       # 4 核心
    memory: 8Gi      # 8GB
```

**持久化存储**: 10Gi PVC

### Redis 缓存

```yaml
replicas: 1
resources:
  requests:
    cpu: 1000m       # 1 核心，支持高并发缓存访问
    memory: 2Gi      # 2GB
  limits:
    cpu: 2000m       # 2 核心
    memory: 4Gi      # 4GB
```

## 资源总计

### Requests (保证资源)
- **CPU**: 7000m (7 核心)
  - Master: 1000m
  - Slaves: 3000m (3 × 1000m)
  - MySQL: 2000m
  - Redis: 1000m
- **Memory**: 10Gi
  - Master: 1Gi
  - Slaves: 3Gi (3 × 1Gi)
  - MySQL: 4Gi
  - Redis: 2Gi

### Limits (最大可用)
- **CPU**: 15000m (15 核心)
- **Memory**: 22Gi

### 集群资源使用率
- **CPU Requests**: 7100m / 8000m = **88%**
- **Memory Requests**: 10310Mi / 12295Mi = **85%**

## 扩展建议

### 水平扩展 (增加 QPS)

根据目标 QPS 调整从节点数量:

| 目标 QPS | 从节点数 | 总 CPU Requests | 总 Memory Requests |
|---------|----------|----------------|-------------------|
| 1000    | 1        | 5000m          | 8Gi               |
| 3000    | 3        | 7000m          | 10Gi              |
| 5000    | 5        | 9000m          | 12Gi              |
| 10000   | 10       | 14000m         | 17Gi              |

```bash
# 扩展到 5 个从节点 (约 5000 QPS)
kubectl scale deployment new-api-slave -n new-api --replicas=5

# 缩减到 2 个从节点 (约 2000 QPS)
kubectl scale deployment new-api-slave -n new-api --replicas=2
```

### 垂直扩展 (提升单节点性能)

修改 `k8s/new-api.yaml` 中的资源配置，然后应用:

```bash
kubectl apply -f k8s/new-api.yaml
```

## 监控和优化

### 查看资源使用情况

```bash
# 查看 Pod 资源使用（需要 metrics-server）
kubectl top pods -n new-api

# 查看节点资源使用
kubectl top nodes

# 查看资源分配情况
kubectl describe node | grep -A 5 "Allocated resources"
```

### 性能调优建议

1. **如果 CPU 使用率持续 > 80%**
   - 增加从节点数量
   - 或提高单节点 CPU limits

2. **如果内存使用率持续 > 85%**
   - 提高内存 limits
   - 检查内存泄漏

3. **如果响应时间变长**
   - 检查 MySQL 慢查询
   - 提高 Redis 缓存命中率
   - 优化数据库索引

4. **如果出现 OOMKilled**
   - 增加内存 limits
   - 检查是否有内存泄漏

## 压力测试

建议使用压测工具验证实际性能:

```bash
# 使用 wrk 进行压测
wrk -t12 -c400 -d30s http://localhost:30000/api/status

# 使用 ab (Apache Bench)
ab -n 10000 -c 100 http://localhost:30000/api/status

# 使用 hey
hey -n 10000 -c 100 http://localhost:30000/api/status
```

在压测期间监控:
```bash
# 实时监控 Pod 资源
watch kubectl top pods -n new-api

# 实时查看日志
kubectl logs -f -n new-api -l app=new-api
```

## 故障排查

### Pod 启动失败

```bash
# 查看 Pod 详情
kubectl describe pod -n new-api <pod-name>

# 查看日志
kubectl logs -n new-api <pod-name>
```

### 资源不足

```bash
# 查看节点资源
kubectl describe nodes

# 如果节点资源不足，降低 requests 或增加节点
```

### 数据库连接问题

```bash
# 测试 MySQL 连接
kubectl run -it --rm mysql-client --image=mysql:8.2 -n new-api -- \
  mysql -h mysql -uroot -p123456 new-api

# 查看 MySQL 日志
kubectl logs -n new-api -l app=mysql
```

## 生产环境建议

在生产环境中，建议:

1. ✅ **使用独立的数据库集群** (如 RDS, CloudSQL)
2. ✅ **使用 Redis 集群**或托管服务
3. ✅ **启用 HPA (Horizontal Pod Autoscaler)**
4. ✅ **配置 PodDisruptionBudget** 确保高可用
5. ✅ **使用 Ingress** 而不是 NodePort
6. ✅ **配置资源监控和告警** (Prometheus + Grafana)
7. ✅ **定期备份数据库**
8. ✅ **使用 NetworkPolicy** 限制网络访问
9. ✅ **修改默认密码和 secrets**
10. ✅ **启用日志聚合** (EFK/ELK)

## 更新记录

- **2025-11-13**: 初始部署，3从节点配置，预期 3000-4000 QPS
