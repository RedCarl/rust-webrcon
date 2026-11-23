# Rust 服务器清档完整需求文档

## 1. 文档目的

本文档用于描述 Rust 服务器“清档并同步地图下载地址”的完整需求，供其他方式重新实现。

本文档描述的是**完整清档流程**，不是单纯的当前地图同步工具。

---

## 2. 背景

当前服务器使用以下技术栈与运行方式：

- 服务端：Rust Dedicated Server
- 管理方式：LinuxGSM
- 运行环境：Docker 容器
- 地图类型：Rust 原版 Procedural Map
- 地图目录：`/data/serverfiles/server/rustserver`
- LinuxGSM 配置文件：`/data/config-lgsm/rustserver/common.cfg`
- 对象存储：S3 兼容对象存储
- 运行时命令下发：通过 WebRCON API

本需求的核心目标是：

1. 每次清档时自动获取一个符合筛选条件的新种子
2. 使用 LinuxGSM 的标准 full wipe 流程完成清档
3. 找到本次清档后新生成的地图文件
4. 将该地图上传到 S3，且对象名与服务端文件名一致
5. 生成玩家可访问的公网下载地址
6. 将 `levelurl` 同步到 LinuxGSM 配置，并尽量同步到运行中的服务器

---

## 3. 总体原则

### 3.1 服务端即真相

地图文件名必须以**服务端实际生成的 `.map` 文件名**为准。

规则：

- 服务端生成什么文件名，就上传什么文件名
- S3 对象名与服务端地图文件名完全一致
- `levelurl` 最终指向同名对象的公网 URL
- 不额外改名，不做别名，不做“latest.map”之类的映射

### 3.2 保留历史地图

S3 中历史地图文件需要保留。

规则：

- 不删除旧地图对象
- 不主动清理历史版本
- 如果同名对象已存在，应视为异常或冲突，由实现方决定是否拒绝覆盖
- 默认建议拒绝覆盖

### 3.3 不做无意义备份

本流程**不需要备份** `common.cfg`。

规则：

- 不要求备份配置
- 不要求失败后回滚配置文件
- 失败时重点是明确日志和停止后续动作

### 3.4 兼容 Docker

因为运行环境是 Docker 容器，实现不应强依赖临时手工安装工具。

规则：

- 不要求必须使用 `aws cli`
- 可使用 `boto3`、`mc`、内置 SDK 或其他稳定方案实现 S3 上传

---

## 4. 范围

### 4.1 本需求包含

- RustMaps 种子选择
- LinuxGSM `common.cfg` 写入 `seed` / `worldsize` / `levelurl`
- 执行 LinuxGSM full wipe
- 定位新生成的 `.map`
- 上传地图到 S3
- 构造公网 `map_url`
- 通过 WebRCON 运行时下发 `server.levelurl`
- 写状态文件

### 4.2 本需求不包含

- 地图内容分析
- 客户端缓存清理
- 历史对象清理
- 回滚到旧地图
- 自动选择“最佳地图”
- 自定义地图上传流程

---

## 5. 种子来源需求

### 5.1 固定使用 RustMaps

清档流程中的地图种子来源固定为 **RustMaps 搜索结果**。

不接受：

- 本地随机种子
- 手工输入种子作为默认流程

允许：

- 显式人工覆盖种子，作为调试或特殊场景
- 但默认正式流程必须走 RustMaps

### 5.2 RustMaps 请求方式

实现需要调用 RustMaps 搜索接口，获取候选种子列表。

参考接口：

- `https://api.rustmaps.com/internal/v1/maps/search?...`

请求体使用一套**固定筛选条件**。

要求：

- 支持使用固定 JSON 作为搜索条件
- 固定筛选条件可来自：
  - 配置文件
  - 常量
  - 外部 JSON 文件
- 该固定筛选条件不要求在清档时动态修改

### 5.3 RustMaps 返回值使用方式

从搜索结果中读取种子列表，使用返回中的：

- `data[].seed`
- `data[].size`

要求：

- 候选种子必须来自 RustMaps 返回结果
- 应优先过滤掉上一次成功清档使用过的 `MAP_SEED`
- 最终从候选列表中选出一个种子用于本次 wipe
- 选取方式可为随机或稳定策略，但必须来自候选结果

### 5.4 失败处理

RustMaps 请求失败、返回为空或无法解析时：

- 终止本次 wipe
- 不继续执行 full wipe
- 不上传地图
- 不更新 `levelurl`

---

## 6. LinuxGSM 配置要求

### 6.1 配置文件路径

固定使用：

- `/data/config-lgsm/rustserver/common.cfg`

### 6.2 需要更新的配置项

清档前需要写入：

- `seed="<本次新种子>"`
- `worldsize="<本次尺寸>"`

地图上传完成后需要写入：

- `levelurl="<map_url>"`

### 6.3 编辑方式要求

要求：

- 如果配置项已存在，则更新
- 如果配置项不存在，则新增
- 保持 LinuxGSM 常见的 `key="value"` 格式

---

## 7. 清档执行要求

### 7.1 必须使用 LinuxGSM full wipe

清档必须通过 LinuxGSM 标准命令执行：

- `./rustserver fw`

不接受：

- 自己拼 stop/start 代替 full wipe
- 跳过 `fw`

### 7.2 清档执行时机

要求：

1. 先写好 `seed` 和 `worldsize`
2. 再执行 `./rustserver fw`

### 7.3 关于运行状态

默认认为 `./rustserver fw` 会负责 LinuxGSM 自己的 wipe/启动流程。

因此：

- 实现方不需要额外设计一套 stop/start 替代它

---

## 8. 地图识别要求

### 8.1 地图目录

默认地图目录：

- `/data/serverfiles/server/rustserver`

也应允许通过配置覆盖。

### 8.2 如何识别本次新地图

实现必须识别**本次清档后新生成的地图文件**，而不是目录里的旧地图。

建议基线方法：

1. 执行 `fw` 前，对目录内现有 `.map` 文件做一次快照
2. 执行 `fw` 后轮询目录
3. 只接受：
   - 新出现的 `.map`
   - 或已有文件但修改时间、大小发生变化
   - 且文件大小必须大于 0

### 8.3 文件名要求

使用服务端实际文件名，例如：

- `Procedural Map.4250.123456789.map`

后续所有环节都必须沿用这个文件名。

### 8.4 超时处理

如果在规定时间内没有识别到本次新地图：

- 终止流程
- 不上传
- 不更新 `levelurl`

---

## 9. S3 上传需求

### 9.1 上传目标

地图需要上传到 S3 兼容对象存储。

### 9.2 对象路径规则

推荐对象 key：

- `rust/maps/<server_identity>/<map_filename>`

例如：

- `rust/maps/rustserver/Procedural Map.4250.123456789.map`

### 9.3 文件命名规则

要求：

- S3 对象名与服务端 `.map` 文件名完全一致
- 不改名
- 不包装
- 不生成别名文件

### 9.4 对象冲突

要求：

- 默认不覆盖历史同名对象
- 如果同名对象已存在，建议终止并报错

### 9.5 技术实现要求

由于运行环境是 Docker 容器，实现不应要求必须安装 `aws cli`。

可接受实现方式：

- `boto3`
- `mc`
- 其他稳定的 S3 SDK

---

## 10. 地图 URL 需求

### 10.1 URL 组成

最终 `map_url` 必须由：

- `PUBLIC_BASE_URL`
- 加上 URL 编码后的 S3 key

拼接得到。

例如：

- `PUBLIC_BASE_URL=https://maps.example.com`
- `S3 key=rust/maps/rustserver/Procedural Map.4250.123456789.map`

得到：

- `https://maps.example.com/rust/maps/rustserver/Procedural%20Map.4250.123456789.map`

### 10.2 公网访问要求

`map_url` 必须能被玩家客户端直接访问。

不允许：

- `127.0.0.1`
- `localhost`
- 容器内地址
- 仅内网可访问的下载地址

### 10.3 URL 与文件名一致性

要求：

- URL 对应的文件名必须与服务端 `.map` 文件名一致

---

## 11. levelurl 同步需求

### 11.1 配置文件写入

地图上传成功后，需要把：

- `levelurl="<map_url>"`

写入：

- `/data/config-lgsm/rustserver/common.cfg`

### 11.2 运行时更新

服务器已经运行后，不需要为了 `levelurl` 再额外重启一次。

要求：

- 不使用二次 stop/start
- 不用为了写 `levelurl` 再重新启动正式服

### 11.3 运行时命令下发

需要通过 WebRCON API 将 `levelurl` 下发给运行中的服务器。

请求方式：

- `POST {WEBRCON_API_URL}/api/servers/{WEBRCON_SERVER_ID}/command`

请求体格式：

```json
{
  "command": "server.levelurl \"<map_url>\"",
  "password": "<WEBRCON_PASSWORD>"
}
```

### 11.4 默认 WebRCON 地址

默认可参考现有更新流程：

- `https://rust-webrcon.carljoy.cn`

### 11.5 失败处理

如果运行时命令下发失败：

- 是否视为整体失败，由实现方决定
- 但至少必须在日志中明确标记失败阶段

建议：

- `common.cfg` 写入成功可视为主流程成功
- WebRCON 下发失败视为“部分成功，需要人工确认”

---

## 12. 状态文件要求

每次清档成功后，需要写入状态文件，至少包含：

- `MAP_FILENAME`
- `MAP_LOCAL_PATH`
- `MAP_S3_KEY`
- `MAP_URL`
- `MAP_SEED`
- `MAP_SIZE`
- `GENERATED_AT`

状态文件用于：

- 记录本次成功结果
- 下次 RustMaps 选种时避开上一次 `MAP_SEED`

---

## 13. 日志要求

至少需要记录以下阶段：

1. 从 RustMaps 获取种子
2. 本次种子和尺寸
3. 写入 `common.cfg`
4. 执行 `./rustserver fw`
5. 等待新地图生成
6. 找到的地图文件名
7. 上传到 S3
8. 校验对象存在
9. 生成 `map_url`
10. 写入 `levelurl`
11. 运行时下发 `server.levelurl`
12. 最终完成

失败时必须明确指出失败阶段。

---

## 14. 错误处理要求

### 14.1 RustMaps 失败

- 终止流程
- 不执行 wipe

### 14.2 `fw` 执行失败

- 终止流程
- 不上传
- 不更新 `levelurl`

### 14.3 地图未生成

- 终止流程
- 不上传
- 不更新 `levelurl`

### 14.4 上传失败

- 终止流程
- 不更新 `levelurl`

### 14.5 `PUBLIC_BASE_URL` 非公网

- 终止流程
- 不上传
- 不更新 `levelurl`

### 14.6 WebRCON 失败

- 日志明确报错
- 是否终止流程由实现方定义，但必须有明确策略

---

## 15. 验收标准

### 15.1 成功场景

执行一次完整 wipe 后，需满足：

1. 从 RustMaps 成功选出一个种子
2. `common.cfg` 中的 `seed` 和 `worldsize` 已更新
3. 成功执行 `./rustserver fw`
4. 成功识别本次新生成的 `.map`
5. S3 中存在对应对象
6. 对象文件名与服务端地图文件名完全一致
7. 生成了正确的公网 `map_url`
8. `common.cfg` 中的 `levelurl` 已更新
9. 已尝试运行时下发 `server.levelurl`
10. 状态文件已正确写入

### 15.2 连续两次 wipe

连续执行两次 wipe 后，需满足：

1. 两次都从 RustMaps 选出合法候选种子
2. 第二次应尽量避免使用第一次的 `MAP_SEED`
3. S3 中保留两份历史地图
4. 不覆盖旧地图对象

---

## 16. 推荐配置项清单

实现建议支持以下配置项：

- `SERVER_IDENTITY`
- `RUST_SERVER_DIR`
- `COMMON_CFG`
- `SERVER_WORLDSIZE`
- `RUSTMAPS_SEARCH_URL`
- `RUSTMAPS_SEARCH_BODY`
- `RUSTMAPS_SEARCH_BODY_FILE`
- `RUSTMAPS_TIMEOUT`
- `S3_HOST`
- `S3_PORT`
- `S3_SECURE`
- `S3_BUCKET`
- `S3_PREFIX`
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION`
- `PUBLIC_BASE_URL`
- `WEBRCON_API_URL`
- `WEBRCON_PASSWORD`
- `WEBRCON_SERVER_ID`
- `STATE_FILE`
- `LOG_FILE`

---

## 17. 一句话总结

该需求的最终目标是：

**每次清档时，固定从 RustMaps 的固定筛选结果中选一个新种子，写入 LinuxGSM `common.cfg`，执行 `./rustserver fw`，识别本次新生成地图，按服务端真实文件名上传到 S3，再把同名对象的公网 URL 写入并下发 `levelurl`，且整个过程不做无意义备份、不为 `levelurl` 再额外重启一次服务器。**
