# ClipBox 便携客户端 / Portable client

解压到可写目录后运行。Windows 双击 `ClipBox.exe`；macOS/Linux 从终端运行 `./ClipBox`。无需安装 ClipBox。

Extract into a writable directory. On Windows, launch `ClipBox.exe`; on macOS/Linux, run `./ClipBox` from a terminal. No ClipBox installer is needed.

- `data/`：应用存储与 WebView 数据；临时目录为 `data/tmp/`。
- `downloads/`：自动下载的房间文件。同名文件追加时间戳，不覆盖已有文件。
- `data/`: application storage and WebView data; temporary files use `data/tmp/`.
- `downloads/`: received room files; duplicate filenames receive a timestamp.

Windows 需要 WebView2 Runtime。Windows、macOS 和 Linux 均提供桌面悬浮窗。Linux 需要 WebKitGTK 4.1、GTK 3 和 Ayatana AppIndicator，剪贴板快捷键使用 X11。macOS 客户端未签名/公证；首次打开受 Gatekeeper 拦截时，请使用系统隐私与安全性设置中对该应用的打开许可。剪贴板快捷键需要辅助功能权限。

Windows requires WebView2 Runtime. Windows, macOS and Linux all provide the floating window. Linux requires WebKitGTK 4.1, GTK 3 and Ayatana AppIndicator, with X11 for clipboard shortcuts. The macOS executable is unsigned and not notarized; if Gatekeeper blocks it, approve this application in Privacy & Security settings. Clipboard shortcuts require Accessibility permission.

客户端的服务端地址在构建时写入：优先使用工作流输入 `server_url`，其次使用仓库变量 `CLIPBOX_SERVER_URL`，均未设置时连接 `http://127.0.0.1:5328`。改变地址后需要重新构建。

The backend URL is embedded at build time: workflow input `server_url`, then the repository variable `CLIPBOX_SERVER_URL`, then `http://127.0.0.1:5328`. Rebuild to change it.
