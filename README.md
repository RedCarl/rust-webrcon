# Rust WebRcon

作者：CarlJoy

这是一个用于管理 Rust 服务器的 Web 面板：

- 前端：Vite + React
- 后端：Go + Gin（提供 API，并直接托管前端打包后的静态文件）
- 数据存储：JSON 文件目录（默认当前工作目录下的 `users/*.json`、`servers/*.json`、`meta.json`）
- 服务器管理：通过 RCON 获取服务器信息；可选通过 `docker exec` 执行 linuxgsm 命令（例如清档/更新）

默认端口：8084

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

## 后端配置文件

后端通过配置文件启动，默认读取 `backend/config.json`（也可以用启动参数 `-config` 指定路径）。

配置项示例：

```json
{
  "bind_addr": "0.0.0.0:8084",
  "jwt_secret": "请替换为足够长的随机字符串",
  "rcon_encryption_key": "请替换为32字节key（base64或64位hex）",
  "data_dir": "/app",
  "docker_bin": "docker"
}
```

说明：

- `jwt_secret` 用于签发/验证登录 token；更换后旧 token 全部失效，需要重新登录
- `rcon_encryption_key` 用于加密/解密保存的 RCON 密码；更换后历史加密数据无法解密，需要重新保存服务器信息
- `data_dir` 会在该目录下创建 `users/`、`servers/`、`meta.json`

## 本地启动（开发）

### 1）启动后端

```bash
cd backend
go mod tidy
go run . -config ./config.json
```

后端默认监听：`http://localhost:8084`

### 2）启动前端（开发服务器）

```bash
npm install
npm run dev
```

前端开发地址：`http://localhost:8888`

前端已配置将 `/api/*` 代理到本地后端（默认 `http://localhost:8084`）。

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

## Docker Exec 操作（linuxgsm）

如果你希望在 Web 面板里触发 `docker exec`（更新/清档），需要满足：

- 运行后端的那台机器上能执行 `docker exec`（需要可访问 Docker daemon）
- 面板里为服务器配置 `container_name`

接口（需要登录后调用）：

- `POST /api/servers/:id/actions/update`
- `POST /api/servers/:id/actions/wipe`

安全限制：

- 后端不允许任意命令执行，只允许固定动作（update/wipe）
- `container_name` 只允许符合容器名称规则的字符

## API 简表

- `POST /api/auth/register`：注册
- `POST /api/auth/login`：登录，返回 `{ token }`
- `GET /api/me`：获取当前用户（Bearer）
- `GET /api/servers`：获取服务器列表（Bearer）
- `POST /api/servers`：新增服务器（Bearer）
- `PUT /api/servers/:id`：更新服务器（Bearer）
- `DELETE /api/servers/:id`：删除服务器（Bearer）
- `POST /api/servers/:id/actions/:action`：执行容器动作（Bearer）

## 许可证

见 [LICENSE](file:///Users/redcarl/projects/rust-webrcon/LICENSE)。
