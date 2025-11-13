#!/bin/bash

# 部署 New-API 到 K8s 集群
echo "🚀 开始部署 New-API 到 Kubernetes 集群..."

# 检查 kubectl 是否可用
if ! command -v kubectl &> /dev/null; then
    echo "❌ 错误: kubectl 命令未找到，请先安装 kubectl"
    exit 1
fi

# 检查集群连接
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ 错误: 无法连接到 Kubernetes 集群，请检查配置"
    exit 1
fi

echo "✅ Kubernetes 集群连接正常"

# 创建命名空间
echo "📦 创建命名空间..."
kubectl apply -f namespace.yaml

# 部署 MySQL
echo "🗄️  部署 MySQL..."
kubectl apply -f mysql.yaml

# 部署 Redis
echo "📮 部署 Redis..."
kubectl apply -f redis.yaml

# 等待 MySQL 和 Redis 就绪
echo "⏳ 等待 MySQL 和 Redis 就绪..."
kubectl wait --for=condition=ready pod -l app=mysql -n new-api --timeout=300s
kubectl wait --for=condition=ready pod -l app=redis -n new-api --timeout=300s

echo "✅ MySQL 和 Redis 已就绪"

# 部署 New-API
echo "🌐 部署 New-API (1主2从)..."
kubectl apply -f new-api.yaml

# 等待 New-API 就绪
echo "⏳ 等待 New-API 部署完成..."
kubectl wait --for=condition=ready pod -l app=new-api -n new-api --timeout=300s

echo ""
echo "✅ 部署完成！"
echo ""
echo "📊 查看部署状态:"
kubectl get pods -n new-api
echo ""
echo "🌐 访问地址:"
echo "  - 本地访问: http://localhost:30000"
echo "  - 集群内访问: http://new-api.new-api.svc.cluster.local:3000"
echo ""
echo "📝 常用命令:"
echo "  - 查看所有资源: kubectl get all -n new-api"
echo "  - 查看日志(主节点): kubectl logs -f -n new-api -l role=master"
echo "  - 查看日志(从节点): kubectl logs -f -n new-api -l role=slave"
echo "  - 进入容器: kubectl exec -it -n new-api <pod-name> -- sh"
echo "  - 删除部署: kubectl delete namespace new-api"
