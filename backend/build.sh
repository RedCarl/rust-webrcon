#!/bin/bash
set -e

echo "=== 开始构建后端 ==="

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# 1. 准备发布目录
echo "[1/2] 准备发布目录..."
cd "$SCRIPT_DIR"
RELEASE_DIR="release"
mkdir -p "$RELEASE_DIR"

# 2. 构建后端 (Linux AMD64)
echo "[2/2] 正在构建后端 (Linux AMD64)..."
# 编译 server 到 release 目录
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$RELEASE_DIR/server" ./cmd/server/main.go

# 创建示例配置文件 (如果不存在)
if [ ! -f "$RELEASE_DIR/config.json" ]; then
    echo '{
  "bind_addr": ":8084",
  "jwt_secret": "change-me-please-to-random-string",
  "data_dir": "./data",
  "docker_bin": "/usr/bin/docker"
}' > "$RELEASE_DIR/config.json"
fi

echo "=== 构建成功！ ==="
echo "后端程序: $SCRIPT_DIR/$RELEASE_DIR/server"
