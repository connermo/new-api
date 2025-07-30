# Docker 部署指南

## 自动构建镜像

本项目使用 GitHub Actions 自动构建 Docker 镜像并推送到 GitHub Container Registry。

### 镜像地址

- **最新版本**: `ghcr.io/connermo/new-api:latest`
- **特定版本**: `ghcr.io/connermo/new-api:v1.0.0`

### 自动构建触发条件

1. **推送到主分支**: 自动构建并推送 `latest` 标签
2. **创建 Release**: 自动构建并推送版本标签
3. **创建 Tag**: 自动构建并推送对应版本

## 快速开始

### 使用 docker-compose (推荐)

```bash
# 克隆仓库
git clone https://github.com/connermo/new-api.git
cd new-api

# 启动服务
docker-compose up -d
```

### 使用 Docker 命令

```bash
# 拉取镜像
docker pull ghcr.io/connermo/new-api:latest

# 运行容器
docker run -d \
  --name new-api \
  -p 3000:3000 \
  -v ./data:/data \
  -v ./logs:/app/logs \
  -e SQL_DSN="root:123456@tcp(mysql:3306)/new-api" \
  -e REDIS_CONN_STRING="redis://redis" \
  ghcr.io/connermo/new-api:latest
```

## 环境变量配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `SQL_DSN` | - | MySQL 连接字符串 |
| `REDIS_CONN_STRING` | - | Redis 连接字符串 |
| `PORT` | 3000 | 服务端口 |
| `SESSION_SECRET` | - | 会话密钥 |
| `TZ` | Asia/Shanghai | 时区设置 |

## 数据持久化

- **数据目录**: `/data` - 数据库文件
- **日志目录**: `/app/logs` - 应用日志

## 健康检查

应用包含健康检查端点：

```bash
curl http://localhost:3000/api/status
```

## 更新镜像

```bash
# 停止服务
docker-compose down

# 拉取最新镜像
docker-compose pull

# 重新启动
docker-compose up -d
```

## 本地开发

如果需要使用本地构建的镜像：

```bash
# 构建本地镜像
docker build -t new-api:local .

# 修改 docker-compose.yml
# image: new-api:local
``` 