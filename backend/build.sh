#!/bin/bash

# 遇到错误立即退出
set -e

echo "=== 开始构建流程 ==="

# 获取当前脚本所在目录的绝对路径
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 1. 构建前端
echo "[1/4] 正在构建前端..."
cd "$PROJECT_ROOT"
# 确保依赖已安装（可选，如果确定已安装可以注释掉）
# npm install 
npm run build

# 2. 准备发布目录
echo "[2/4] 准备发布目录..."
cd "$SCRIPT_DIR"
RELEASE_DIR="release"
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

# 3. 构建后端 (Linux AMD64)
echo "[3/4] 正在构建后端 (Linux AMD64)..."
# CGO_ENABLED=0 确保静态链接，避免 glibc 版本依赖问题
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$RELEASE_DIR/server" cmd/server/main.go

# 4. 打包最终产物
echo "[4/4] 正在打包..."
# 创建临时目录用于打包结构
PACKAGE_NAME="rust-panel-linux-amd64"
PACKAGE_DIR="$RELEASE_DIR/$PACKAGE_NAME"
mkdir -p "$PACKAGE_DIR"

# 移动二进制文件
mv "$RELEASE_DIR/server" "$PACKAGE_DIR/"
chmod +x "$PACKAGE_DIR/server"

# 复制前端静态资源
cp -r "$PROJECT_ROOT/dist" "$PACKAGE_DIR/"

# 创建示例配置文件（如果不存在）
if [ ! -f "$PACKAGE_DIR/config.json" ]; then
    echo '{
  "bind_addr": ":8084",
  "jwt_secret": "change-me-please",
  "data_dir": "./data"
}' > "$PACKAGE_DIR/config.json"
fi

# 创建启动脚本
cat > "$PACKAGE_DIR/start.sh" << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
./server
EOF
chmod +x "$PACKAGE_DIR/start.sh"

# 压缩
cd "$RELEASE_DIR"
tar -czf "$PACKAGE_NAME.tar.gz" "$PACKAGE_NAME"

# 清理临时目录
rm -rf "$PACKAGE_NAME"

echo "=== 构建成功！ ==="
echo "产物路径: $SCRIPT_DIR/$RELEASE_DIR/$PACKAGE_NAME.tar.gz"
