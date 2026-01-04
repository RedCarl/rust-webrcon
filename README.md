# Rust WebRcon

作者：CarlJoy

这是一个用于管理 Rust 服务器的 Web 面板：

- 前端：Vite + React
- 后端：Go + Gin（提供 API，并直接托管前端打包后的静态文件）
- 数据库：MySQL 5.7
- 服务器管理：通过 RCON 获取服务器信息；可选通过 `docker exec` 执行 linuxgsm 命令（例如清档/更新）

默认端口：3000

## 功能概览

- 用户注册/登录（JWT）
- 服务器列表（增删改查）
- 一键连接到指定服务器（WebSocket RCON）
- 可选：对配置了容器名称的服务器执行
  - 更新：`docker exec -u linuxgsm <container> ./rustserver u`
  - 清档：`docker exec -u linuxgsm <container> ./rustserver w`

## 目录结构

- `src/`：前端代码
- `backend/`：后端 Go 服务
- `dist/`：前端打包产物（本地 `npm run build` 生成；Docker 构建时也会生成）

## 环境变量

后端启动必须提供以下环境变量：

- `DATABASE_URL`：MySQL DSN（go-sql-driver/mysql 格式）
  - 示例：`user:pass@tcp(127.0.0.1:3306)/rust?parseTime=true&charset=utf8mb4&loc=Local`
- `JWT_SECRET`：JWT 签名密钥（任意足够长的随机字符串）
- `RCON_ENCRYPTION_KEY`：用于加密数据库中保存的 RCON 密码，必须为 32 字节（base64 或 64 位 hex）
- `BIND_ADDR`：监听地址（可选，默认 `0.0.0.0:3000`）
- `DOCKER_BIN`：docker 可执行文件名/路径（可选，默认 `docker`）

说明：

- `JWT_SECRET` 用于签发/验证登录 token；更换后旧 token 全部失效，需要重新登录
- `RCON_ENCRYPTION_KEY` 用于加密/解密服务器的 RCON 密码；更换后历史加密数据无法解密，需要重新保存服务器信息

## 本地启动（开发）

### 1）启动后端

```bash
cd backend

export DATABASE_URL="user:pass@tcp(127.0.0.1:3306)/rust?parseTime=true&charset=utf8mb4&loc=Local"
export JWT_SECRET="$(openssl rand -hex 32)"
export RCON_ENCRYPTION_KEY="$(openssl rand -base64 32)"

go mod tidy
go run .
```

后端默认监听：`http://localhost:3000`

### 2）启动前端（开发服务器）

```bash
npm install
npm run dev
```

前端开发地址：`http://localhost:8888`

前端已配置将 `/api/*` 代理到本地后端（默认 `http://localhost:3000`）。

## 生产构建（本地）

### 构建前端

```bash
npm install
npm run build
```

产物会输出到 `dist/`。

### 编译后端

```bash
cd backend
go mod tidy
go build ./...
```

## Docker 运行

本项目提供根目录 [Dockerfile](file:///Users/redcarl/projects/rust-webrcon/Dockerfile)，镜像内包含：

- 后端二进制（Go）
- 前端静态文件（`dist/`）

### 构建镜像

```bash
docker build -t rust-panel:latest .
```

### 运行容器

```bash
docker run -d \
  --name rust-panel \
  -p 3000:3000 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e DATABASE_URL="user:pass@tcp(db:3306)/rust?parseTime=true&charset=utf8mb4&loc=Local" \
  -e JWT_SECRET="your-jwt-secret" \
  -e RCON_ENCRYPTION_KEY="your-32-bytes-key" \
  rust-panel:latest
```

浏览器访问：`http://localhost:3000/#/login`

## Docker Exec 操作（linuxgsm）

如果你希望在 Web 面板里触发 `docker exec`（更新/清档），需要满足：

- 运行后端的那台机器上能执行 `docker exec`（需要可访问 Docker daemon）
- 如果后端运行在容器里，需要挂载宿主机的 `docker.sock`：`-v /var/run/docker.sock:/var/run/docker.sock`
- 面板里为服务器配置 `container_name`

接口（需要登录后调用）：

- `POST /api/servers/:id/actions/update`
- `POST /api/servers/:id/actions/wipe`

安全限制：

- 后端不允许任意命令执行，只允许固定动作（update/wipe）
- `container_name` 只允许符合容器名称规则的字符

安全提示：

- 把 `docker.sock` 挂进容器等同于给容器很高的宿主机权限，请只在可信环境使用，并尽量限制面板的访问范围

## API 简表

- `POST /api/auth/register`：注册
- `POST /api/auth/login`：登录，返回 `{ token }`
- `GET /api/me`：获取当前用户（Bearer）
- `GET /api/servers`：获取服务器列表（Bearer）
- `POST /api/servers`：新增服务器（Bearer）
- `PUT /api/servers/:id`：更新服务器（Bearer）
- `DELETE /api/servers/:id`：删除服务器（Bearer）
- `POST /api/servers/:id/actions/:action`：执行容器动作（Bearer）

## 通过 GitHub Actions 自动发布镜像（GHCR）

工作流文件：`.github/workflows/docker-publish.yml`

触发条件：

- 推送到 `main`
- 推送 tag `v*`
- 手动触发 `workflow_dispatch`

镜像地址：

- `ghcr.io/<owner>/rust-panel:latest`

## 许可证

见 [LICENSE](file:///Users/redcarl/projects/rust-webrcon/LICENSE)。
