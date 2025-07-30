#!/bin/bash

# 版本发布脚本
# 使用方法: ./scripts/release.sh <version>
# 例如: ./scripts/release.sh v1.0.0

set -e

if [ $# -eq 0 ]; then
    echo "使用方法: $0 <version>"
    echo "例如: $0 v1.0.0"
    exit 1
fi

VERSION=$1

# 验证版本格式
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "错误: 版本格式不正确，请使用 vX.Y.Z 格式"
    echo "例如: v1.0.0"
    exit 1
fi

echo "🚀 开始发布版本: $VERSION"

# 更新VERSION文件
echo "📝 更新VERSION文件..."
echo "$VERSION" > VERSION

# 提交更改
echo "📦 提交更改..."
git add VERSION
git commit -m "chore: 发布版本 $VERSION"

# 创建标签
echo "🏷️  创建Git标签..."
git tag -a "$VERSION" -m "Release $VERSION"

# 推送到远程仓库
echo "📤 推送到远程仓库..."
git push connermo main
git push connermo "$VERSION"

echo "✅ 版本 $VERSION 发布完成！"
echo ""
echo "📋 下一步操作："
echo "1. 访问 https://github.com/connermo/new-api/releases"
echo "2. 编辑 $VERSION 标签，添加发布说明"
echo "3. 发布后GitHub Actions将自动构建Docker镜像"
echo ""
echo "🐳 Docker镜像将在以下地址可用："
echo "   ghcr.io/connermo/new-api:$VERSION"
echo "   ghcr.io/connermo/new-api:latest" 