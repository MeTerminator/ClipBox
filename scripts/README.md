# Python 房间测试端

`room_client.py` 是独立的房间成员，使用与 Tauri 相同的 HTTP/WebSocket 协议。使用 uv 管理 Python 3.10+ 和脚本依赖，无需手动创建或激活虚拟环境。默认连接 `https://box.mett.top:444`，不依赖前端或 Tauri 开发服务器。

在项目根目录执行：

```bash
uv run scripts/room_client.py
```

启动后输入已存在的 5 位房间 ID。也可直接指定：

```bash
uv run scripts/room_client.py 12345
# 有密码的房间：隐藏输入，不把密码放在命令行参数中
uv run scripts/room_client.py 12345 --ask-password
# 测试本地服务
uv run scripts/room_client.py 12345 --api http://127.0.0.1:5328
```

请先在 Tauri 中创建或加入房间并保持在线，再运行脚本。房间最后一个在线成员退出时，服务端会立即删除房间。脚本退出会断开它自己的连接；断线时会提示并退出，可重新运行加入。

加入后会输出历史聊天、剪贴板消息和文件 URL，并持续输出新消息。支持命令：

| 命令 | 功能 |
| --- | --- |
| `/get` | 输出最新房间剪贴板文本；`null` 表示尚无剪贴板消息 |
| `/set 测试文本` | 发送新的房间剪贴板文本，广播回显后确认 |
| `/set-file /绝对路径/text.txt` | 将 UTF-8 文本文件内容作为剪贴板发送，支持多行 |
| `/chat 普通聊天` | 发送普通聊天，不改变房间剪贴板 |
| `/history` | 输出已加载及实时收到的所有消息 |
| `/files` | 输出文件消息、名称、大小和完整下载 URL |
| `/download 42` | 为文件消息 ID 42 输出带成员令牌的 curl 下载命令 |
| `/help` | 显示帮助 |
| `/quit` | 退出；也可按 Ctrl+C |

房间剪贴板来自最新的 `kind: text, source: clipboard` 消息。脚本不会直接读取或修改本机系统剪贴板。服务端不接受空白文本，因此 `/set` 不能用来清空剪贴板。

文件 URL 需要当前房间成员的 Bearer 令牌，直接在浏览器打开不一定能下载。`/download` 只输出命令，不自动下载；复制该命令到另一终端运行即可。该命令含成员令牌，请勿公开分享。默认输出聊天及文件 URL 时不显示令牌。

## 与 Tauri 双向测试

1. 保持 Tauri 在线并加入同一房间，确保全局快捷键权限已授权。
2. 在文本编辑器选择文字并按 Command+C（Windows/Linux 用 Ctrl+C）。脚本应显示新的 `[剪贴板]` 消息，输入 `/get` 应得到相同文本。
3. 在脚本输入 `/set Python 发给 Tauri 的文本`，等 `[本脚本回显确认]` 出现，再在文本编辑器按 Command+V，确认粘贴出新文本。Tauri 仅在按粘贴快捷键时写入系统剪贴板。
4. 输入 `/chat 这是一条普通聊天`，确认 Tauri 房间显示聊天，`/get` 的结果不变。
5. 用 Tauri 或浏览器上传一个文件；脚本应输出文件名和完整 URL。输入 `/files` 再次查看，使用 `/download 消息ID` 获得下载命令。
6. 切换中英文输入法，重复复制/粘贴并观察客户端是否稳定。

## 脚本集成测试

测试仅使用临时本地服务、SQLite 和测试房间，不访问默认远程服务。先在项目根目录构建服务（需 Go 和已构建的 `www`）：

```bash
go build -o /tmp/clipbox-room-test-server .
uv run scripts/test_room_client.py --server /tmp/clipbox-room-test-server
```

验证加入、200+ 条历史分页、读取/修改剪贴板、实时聊天、文件 URL/授权下载及交互命令。临时服务与数据会在测试结束后清理。
