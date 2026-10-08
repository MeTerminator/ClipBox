# /// script
# requires-python = ">=3.10"
# dependencies = ["websocket-client>=1.8,<2"]
# ///
"""Interactive ClipBox room peer for testing a Tauri desktop client (Python 3.10+)."""

import argparse
import getpass
import json
import platform
import queue
import shlex
import sys
import threading
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import urljoin, urlsplit, urlunsplit
from urllib.request import Request, urlopen

try:
    import websocket

    if not hasattr(websocket, "create_connection"):
        raise ImportError("wrong websocket package")
except ImportError:
    sys.exit("请用 uv 运行脚本：uv run scripts/room_client.py")


class RoomClient:
    def __init__(self, origin, room_id, timeout=15):
        parts = urlsplit(origin)
        if (
            parts.scheme not in ("http", "https")
            or not parts.netloc
            or parts.path not in ("", "/")
            or parts.query
            or parts.fragment
            or parts.username
        ):
            raise ValueError(
                "--api 必须是 http(s) 服务根地址，例如 https://box.mett.top:444"
            )
        if len(room_id) != 5 or not room_id.isascii() or not room_id.isdigit():
            raise ValueError("房间 ID 必须是 5 位数字")
        self.origin = origin.rstrip("/")
        self.room_id = room_id
        self.timeout = timeout
        self.token = ""
        self.ws = None
        self.messages = {}
        self.lock = threading.RLock()
        self.stopped = threading.Event()
        self.member_id = None

    def url(self, path):
        return urljoin(self.origin + "/", path)

    def request(self, path, body=None):
        headers = {"Accept": "application/json", "User-Agent": "ClipBox-Python-Test/1"}
        if self.token:
            headers["Authorization"] = "Bearer " + self.token
        data = None
        if body is not None:
            headers["Content-Type"] = "application/json"
            data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        try:
            with urlopen(
                Request(self.url(path), data=data, headers=headers),
                timeout=self.timeout,
            ) as response:
                return json.load(response)
        except HTTPError as error:
            try:
                reason = json.load(error).get("error", str(error))
            except (ValueError, AttributeError):
                reason = str(error)
            raise RuntimeError(f"HTTP {error.code}: {reason}") from error

    def join(self, password, nickname):
        session = self.request(
            f"/api/rooms/{self.room_id}/join",
            {
                "password": password,
                "nickname": nickname,
                "device": "Python test client",
                "os": platform.system(),
                "browser": "Python",
            },
        )
        self.token = session["token"]
        self.member_id = session["room"]["current_member_id"]
        parts = urlsplit(self.url(f"/api/rooms/{self.room_id}/ws"))
        ws_url = urlunsplit(
            (
                "wss" if parts.scheme == "https" else "ws",
                parts.netloc,
                parts.path,
                "",
                "",
            )
        )
        self.ws = websocket.create_connection(
            ws_url, origin=self.origin, timeout=self.timeout, enable_multithread=True
        )
        self.send({"type": "auth", "token": self.token})
        event = json.loads(self.ws.recv())
        if event.get("type") != "ready":
            raise RuntimeError(
                f"WebSocket 认证失败：{event.get('code', event.get('type'))}"
            )
        self.ws.settimeout(None)
        for message in event.get("messages", []):
            self.remember(message)
        # ready contains only the first 200 messages. Read all pages so /get
        # also finds clipboard changes in older, active rooms with long histories.
        self.refresh_history()
        room = event["room"]
        print(
            f"已加入房间 {self.room_id}：{room.get('name', '')}；成员 ID {self.member_id}",
            flush=True,
        )

    def refresh_history(self):
        after = 0
        while True:
            batch = self.request(
                f"/api/rooms/{self.room_id}/messages?after={after}"
            ).get("messages", [])
            for message in batch:
                self.remember(message)
            if len(batch) < 200:
                return
            next_id = max(message["id"] for message in batch)
            if next_id <= after:
                raise RuntimeError("历史消息分页未前进")
            after = next_id

    def remember(self, message):
        with self.lock:
            fresh = message["id"] not in self.messages
            self.messages[message["id"]] = message
            return fresh

    def history(self):
        with self.lock:
            return sorted(self.messages.values(), key=lambda message: message["id"])

    def clipboard(self):
        return next(
            (
                message["text"]
                for message in reversed(self.history())
                if message.get("kind") == "text"
                and message.get("source") == "clipboard"
            ),
            None,
        )

    def show_clipboard(self):
        print(
            "当前房间剪贴板：" + json.dumps(self.clipboard(), ensure_ascii=False),
            flush=True,
        )

    def display(self, message):
        sender = message.get("sender", {}).get("nickname", "未知成员")
        own = (
            " [本脚本回显确认]"
            if message.get("sender", {}).get("id") == self.member_id
            else ""
        )
        label = "剪贴板" if message.get("source") == "clipboard" else "聊天"
        prefix = f"[{message.get('created_at', '')}] #{message['id']} {sender}{own}"
        if message.get("kind") == "file":
            file = message.get("file") or {}
            print(
                f"{prefix} [文件] {json.dumps(file.get('name', ''), ensure_ascii=False)} ({file.get('size', 0)} 字节)\n  URL: {self.url(file.get('download_url', ''))}",
                flush=True,
            )
        else:
            # JSON quoting keeps newlines and terminal control characters readable.
            print(
                f"{prefix} [{label}] {json.dumps(message.get('text', ''), ensure_ascii=False)}",
                flush=True,
            )

    def send(self, command):
        if self.stopped.is_set() or self.ws is None:
            raise RuntimeError("房间连接已关闭")
        self.ws.send(json.dumps(command, ensure_ascii=False))

    def send_text(self, text, source):
        if not text.strip():
            raise ValueError("服务端不接受空白文本；不能用空文本清空房间剪贴板")
        self.send(
            {
                "type": "send",
                "message": {"kind": "text", "source": source, "text": text},
            }
        )
        print("已提交，服务端广播回显后确认。", flush=True)

    def receive(self):
        try:
            while not self.stopped.is_set():
                raw = self.ws.recv()
                if not raw:
                    raise RuntimeError("服务端关闭连接")
                event = json.loads(raw)
                kind = event.get("type")
                if kind == "message":
                    if self.remember(event["message"]):
                        self.display(event["message"])
                elif kind == "error":
                    print(f"服务端错误：{event.get('code')}", flush=True)
                elif kind == "room_deleted":
                    raise RuntimeError("房间已删除")
        except (
            OSError,
            RuntimeError,
            ValueError,
            KeyError,
            TypeError,
            websocket.WebSocketException,
        ) as error:
            if not self.stopped.is_set():
                print(f"连接已断开：{error}；请重新运行脚本加入房间。", flush=True)
        finally:
            self.stopped.set()

    def heartbeat(self):
        while not self.stopped.wait(20):
            try:
                self.send({"type": "ping"})
            except (OSError, RuntimeError, websocket.WebSocketException):
                self.close()
                return

    def close(self):
        self.stopped.set()
        if self.ws:
            self.ws.close()


HELP = """命令：
  /get                  获取当前房间剪贴板（不会读取本机系统剪贴板）
  /set 文本             修改房间剪贴板；Tauri 收到后按 Command+V 验证
  /set-file 路径        将 UTF-8 文本文件的内容发送为房间剪贴板
  /chat 文本            发送普通聊天，不修改房间剪贴板
  /history              输出已收到的所有聊天、剪贴板和文件消息
  /files                仅输出文件消息及下载 URL
  /download 消息ID      输出带授权的 curl 下载命令（令牌只在此命令显示）
  /help                 显示帮助
  /quit                 退出
Tauri 按 Command+C 后，脚本应实时输出 [剪贴板] 消息。"""


def interactive(client):
    print(HELP, flush=True)
    for message in client.history():
        client.display(message)
    client.show_clipboard()
    threading.Thread(target=client.receive, daemon=True).start()
    threading.Thread(target=client.heartbeat, daemon=True).start()
    lines = queue.Queue()

    def read_input():
        for line in sys.stdin:
            lines.put(line.rstrip("\r\n"))
        lines.put(None)

    threading.Thread(target=read_input, daemon=True).start()
    while not client.stopped.is_set():
        try:
            line = lines.get(timeout=0.2)
        except queue.Empty:
            continue
        if line is None or line == "/quit":
            return
        command, _, value = line.partition(" ")
        try:
            if command == "/get":
                client.show_clipboard()
            elif command in ("/set", "/chat"):
                client.send_text(value, "clipboard" if command == "/set" else "user")
            elif command == "/set-file":
                client.send_text(
                    Path(value).expanduser().read_text(encoding="utf-8"), "clipboard"
                )
            elif command in ("/history", "/files"):
                for message in client.history():
                    if command == "/history" or message.get("kind") == "file":
                        client.display(message)
            elif command == "/download":
                message = next(
                    (
                        item
                        for item in client.history()
                        if item["id"] == int(value) and item.get("file")
                    ),
                    None,
                )
                if not message:
                    raise ValueError("未找到该文件消息")
                print(
                    shlex.join(
                        [
                            "curl",
                            "--fail",
                            "--location",
                            "--remote-name",
                            "--remote-header-name",
                            "-H",
                            "Authorization: Bearer " + client.token,
                            client.url(message["file"]["download_url"]),
                        ]
                    ),
                    flush=True,
                )
            elif command == "/help":
                print(HELP, flush=True)
            elif line:
                print("未知命令。输入 /help 查看帮助。", flush=True)
        except (
            ValueError,
            OSError,
            RuntimeError,
            websocket.WebSocketException,
        ) as error:
            print(f"操作失败：{error}", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("room", nargs="?", help="已存在的 5 位房间 ID")
    parser.add_argument("--api", default="https://box.mett.top:444", help="服务根地址")
    parser.add_argument("--nickname", default="Python 测试端")
    parser.add_argument(
        "--ask-password", action="store_true", help="安全提示输入房间密码"
    )
    args = parser.parse_args()
    client = None
    try:
        room = args.room or input("房间 ID：").strip()
        password = getpass.getpass("房间密码：") if args.ask_password else ""
        client = RoomClient(args.api, room)
        client.join(password, args.nickname)
        interactive(client)
        return 0
    except KeyboardInterrupt:
        print("\n已退出。")
        return 0
    except (
        ValueError,
        RuntimeError,
        OSError,
        URLError,
        websocket.WebSocketException,
    ) as error:
        print(f"失败：{error}", file=sys.stderr)
        return 1
    finally:
        if client:
            client.close()


if __name__ == "__main__":
    sys.exit(main())
