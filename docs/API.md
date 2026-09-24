# 后端 API 契约

默认地址为 `http://127.0.0.1:5328`。普通错误响应格式为：

```json
{"error":"错误说明"}
```

`count` 是最大取件次数，`expire` 是从创建时间起算的有效秒数；省略或传 `0` 时分别为 `1000` 和 `86400`。

### `GET /api/config`

返回前端上传路由选择所需的公开配置：`site_url` 和 `upload_direct_first`。该接口不包含数据库等服务端敏感配置。

### `GET /api/clip/upload/__direct_probe__`

专用于浏览器判断源站上传 API 是否可达，成功返回 `200 {"direct_upload":true}`。该端点固定允许跨域。

## Clip

### `POST /api/clip/create`

使用表单字段创建文本或链接：`content`（必填）、`link=yes`（链接时）、`count`、`expire`。成功返回 `200 {"code":"12345"}`。

### `GET /api/clip/:code/info`

读取元数据但不扣减次数。返回类型、到期时间、剩余次数和最大次数；文件额外返回文件名与大小。

### `POST /api/clip/:code/resolve`

扣减一次取件次数并返回结构化内容。文本和链接包含 `content`；文件包含 `filename`、`size` 和 `download_url`。

### `GET /api/clip/:code`

兼容接口。扣减一次次数后，链接跳转到目标地址，文本跳转到 `/api/text/:sha1`，文件跳转到 `/api/file/:sha1/:filename`。

### `GET /api/text/:sha1` 与 `GET /api/file/:sha1/:filename`

读取仍被有效 Clip 引用的内容。文件响应包含安全的 `Content-Disposition` 和 `X-Content-Type-Options: nosniff`。

## 分片上传

### `POST /api/clip/upload/init`

```json
{"filename":"report.pdf","size":1234,"sha1":"40位小写十六进制","count":1000,"expire":86400}
```

若物理内容已存在，返回 `instant_upload: true`、取件码和 URL。否则返回上传 ID、分片大小、总分片数、已有分片、过期时间和建议并发数。

### `GET /api/clip/upload/:uploadID`

返回当前上传状态并刷新会话活跃时间。

### `PUT /api/clip/upload/:uploadID/:chunk`

请求体是该分片的原始字节。除最后一片外长度必须精确等于服务端声明的 `chunk_size`。

### `POST /api/clip/upload/:uploadID/complete`

```json
{"filename":"report.pdf","count":1000,"expire":86400}
```

服务端重新组装并验证大小及 SHA-1。缺少分片返回 `409`，摘要不匹配返回 `422`，会话过期返回 `410`。

## 分享房间

### 身份与授权

`POST /api/rooms` 创建房间，`POST /api/rooms/:roomID/join` 加入。房间与取件码共用同一个 5 位数字 ID 命名空间，服务端会原子保留 ID，避免活动房间和活动取件码重复。身份字段为 `nickname`、`device`、`os`、`browser`；创建请求还可包含 `name`；加入请求可在房间设置密码后包含 `password`（小于 64 个 Unicode 字符，支持任意字符）。成功响应中的 `token` 只返回一次，后续 HTTP 请求使用：

```text
Authorization: Bearer <token>
```

### 管理和历史

- `GET /api/rooms/:roomID`：房间和成员状态。
- `PATCH /api/rooms/:roomID`：任意成员修改 `{ "name": "..." }`。
- `PATCH /api/rooms/:roomID/me`：成员修改自己的 `{ "nickname": "..." }`。
- `PUT /api/rooms/:roomID/password`：任意成员设置或清空 `{ "password": "..." }`；空字符串清空密码。
- `DELETE /api/rooms/:roomID`：仅房主删除房间。房主断开后若仍有成员在线，房主会按加入顺序自动转让给下一位成员。
- `GET /api/rooms/:roomID/messages?after=<id>`：按 ID 升序返回最多 200 条后续消息。
- `GET /api/rooms/:roomID/messages/:messageID/file`：下载房间内的文件消息。

### WebSocket `GET /api/rooms/:roomID/ws`

连接后的第一帧必须认证：

```json
{"type":"auth","token":"..."}
```

发送文本或已上传文件：

```json
{"type":"send","message":{"kind":"text","source":"user","text":"hello"}}
{"type":"send","message":{"kind":"file","source":"user","file_code":"12345"}}
```

`source` 可为 `user` 或 `clipboard`。当前剪贴板协议只接受 `kind:"text"`；以 `source:"clipboard"` 发送文件会被服务端拒绝。服务端事件包括 `ready`、`message`、`presence`、`member_updated`、`room_updated`、`room_deleted`、`pong` 和 `error`。消息先持久化再广播，断线后可用历史接口补齐。

### 生命周期

房间没有在线 WebSocket 后会立即销毁，房间消息同时删除；房间文件会保留 1 天后由清理任务删除。房主退出但房间仍有其他成员时，房主身份按成员加入顺序自动转让。
