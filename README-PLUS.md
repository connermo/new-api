# New API Plus 版本说明

本仓库是 [New API](https://github.com/Calcium-Ion/new-api) 的增强版本，在官方版本的基础上增加了以下功能。

## 版本命名规则

- **官方版本**: 0.10.3
- **Plus 版本**: 0.10.3-plus.X
  - 基于官方版本号，后缀标识自定义版本
  - 当官方发布新版本时，plus 后缀重新从 1 开始

## 相比官方版本的新增功能

### 🎯 核心功能增强

#### 1. 操练场 (Playground) 性能指标显示
- 为每条 AI 消息显示详细的性能数据
- 实时显示：
  - **输入 tokens**: 提示词的 token 数量
  - **输出 tokens**: 回复内容的 token 数量
  - **吞吐量**: tokens/秒，反映模型响应速度
  - **首字延迟**: 流式请求中首个 token 的响应时间
- 显示格式：`输入:100 输出:50 | 吞吐:23.5 tokens/s | 首字:0.8s`
- 仅在助手消息完成后显示，不影响使用体验

#### 2. 使用日志搜索增强

##### 渠道名称显示
- 使用日志中显示渠道名称而非 ID
- 提升可读性，无需记忆渠道 ID
- 鼠标悬停显示完整信息（渠道名称 + ID）

##### 渠道名称搜索
- 支持按渠道名称搜索日志（而非仅支持 ID）
- 支持模糊搜索和精确搜索
- 例如：搜索 "OpenAI" 可找到所有包含该关键词的渠道日志

##### 全字段模糊搜索
- 对所有可搜索字段启用模糊搜索功能
- 支持的字段：
  - 模型名称：搜索 "gpt" 可找到所有 GPT 系列模型
  - 用户名：部分匹配用户名
  - 令牌名称：部分匹配令牌
  - 分组：模糊匹配分组名称
  - 渠道：按渠道名称模糊搜索
- 修复充值记录搜索的通配符问题

#### 3. 模型支持扩展

##### Qwen3-Embedding 支持
- 添加 Qwen3-Embedding 模型完整支持
- 完善模型配置和计费规则

#### 4. Token 时间限制
- 新增 token 时间限制功能
- 支持设置 token 的有效期
- 提供更精细的访问控制和安全管理

### 🚀 部署与 CI/CD

#### 1. GitHub Actions 自动化

##### Docker 镜像自动构建
- 推送代码自动触发 Docker 镜像构建
- 自动发布到 GitHub Container Registry (ghcr.io)
- 支持多架构构建（AMD64、ARM64）

##### Latest 标签自动更新
- 新版本发布时自动更新 `latest` 标签
- 简化 Docker 部署流程

##### CI 流程优化
- 修复 tag 推送时的无效标签生成问题
- 优化 Docker 登录流程，提升构建稳定性

#### 2. Kubernetes 部署配置
- 提供完整的 K8s 部署配置文件
- 支持容器化编排部署
- 包含 Deployment、Service、Ingress 等资源配置

#### 3. 版本发布脚本
- 自动化版本发布流程
- 简化版本管理和发布操作
- 确保版本号一致性

## 安装使用

### Docker 部署

使用 Plus 版本的 Docker 镜像：

```bash
docker pull ghcr.io/connermo/new-api:latest
```

或指定具体版本：

```bash
docker pull ghcr.io/connermo/new-api:v0.10.3-plus.2
```

### Docker Compose

参考项目中的 `docker-compose.yml` 文件，将镜像地址修改为：

```yaml
services:
  new-api:
    image: ghcr.io/connermo/new-api:latest
    # ... 其他配置
```

## 更新计划

- 定期同步上游官方版本
- 持续优化用户体验
- 根据使用反馈添加新功能

## 贡献

欢迎提交 Issue 和 Pull Request！

## 致谢

感谢 [New API](https://github.com/Calcium-Ion/new-api) 官方项目提供的优秀基础。

## 许可证

本项目继承原项目的 AGPL-3.0 许可证。
