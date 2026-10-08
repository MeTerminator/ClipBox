# /// script
# requires-python = ">=3.10"
# dependencies = ["websocket-client>=1.8,<2"]
# ///
"""Integration check against an isolated, temporary local ClipBox server."""

import argparse
import contextlib
import hashlib
import io
import json
import os
import socket
import subprocess
import tempfile
import threading
import time
from pathlib import Path
from urllib.request import Request, urlopen

import websocket
from room_client import RoomClient


def wait_for(check, timeout=10):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        if check():
            return
        time.sleep(0.05)
    raise AssertionError("等待测试结果超时")


def run(server):
    with tempfile.TemporaryDirectory(prefix="clipbox-room-test-") as folder:
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            port = probe.getsockname()[1]
        origin = f"http://127.0.0.1:{port}"
        env = dict(
            os.environ,
            ADDRESS=f"127.0.0.1:{port}",
            DATA_DIR=folder,
            CONFIG_FILE=folder + "/config.json",
            DATABASE_DRIVER="sqlite",
            DATABASE_DSN=folder + "/clipbox.db",
            NO_PROXY="127.0.0.1,localhost",
            no_proxy="127.0.0.1,localhost",
        )
        with open(folder + "/server.log", "w+") as log:
            process = subprocess.Popen(
                [str(Path(server).resolve())],
                cwd=folder,
                env=env,
                stdout=log,
                stderr=log,
            )
            owner = client = None
            try:

                def ready():
                    try:
                        with urlopen(origin + "/api/config", timeout=0.3) as response:
                            return response.status == 200
                    except OSError:
                        return False

                wait_for(ready)
                api = RoomClient(origin, "00000")
                session = api.request(
                    "/api/rooms", {"name": "Python integration", "nickname": "owner"}
                )
                room_id = session["room"]["id"]
                api.room_id, api.token = room_id, session["token"]
                owner = websocket.create_connection(
                    origin.replace("http:", "ws:") + f"/api/rooms/{room_id}/ws",
                    origin=origin,
                    timeout=10,
                )
                owner.send(json.dumps({"type": "auth", "token": api.token}))
                assert json.loads(owner.recv())["type"] == "ready"

                def send_owner(message):
                    owner.send(json.dumps({"type": "send", "message": message}))
                    while True:
                        event = json.loads(owner.recv())
                        if event["type"] == "message":
                            return event["message"]
                        assert event["type"] != "error", event

                # Exercise >200 messages: the ready snapshot alone is insufficient.
                for number in range(201):
                    send_owner(
                        {"kind": "text", "source": "user", "text": f"history {number}"}
                    )
                send_owner(
                    {"kind": "text", "source": "clipboard", "text": "旧剪贴板\n中文"}
                )
                payload = b"room test file\n"
                upload = api.request(
                    "/api/clip/upload/init",
                    {
                        "filename": "test.txt",
                        "size": len(payload),
                        "sha1": hashlib.sha1(payload).hexdigest(),
                    },
                )
                with urlopen(
                    Request(
                        origin + f"/api/clip/upload/{upload['upload_id']}/0",
                        data=payload,
                        method="PUT",
                    ),
                    timeout=10,
                ):
                    pass
                completed = api.request(
                    f"/api/clip/upload/{upload['upload_id']}/complete",
                    {"filename": "test.txt"},
                )
                file_message = send_owner(
                    {"kind": "file", "source": "user", "file_code": completed["code"]}
                )
                client = RoomClient(origin, room_id)
                with contextlib.redirect_stdout(io.StringIO()):
                    client.join("", "test peer")
                assert len(client.history()) == 203
                assert client.clipboard() == "旧剪贴板\n中文"
                file_url = client.url(file_message["file"]["download_url"])
                assert file_url.startswith(origin + "/api/rooms/")
                with urlopen(
                    Request(
                        file_url, headers={"Authorization": "Bearer " + client.token}
                    ),
                    timeout=10,
                ) as response:
                    assert response.read() == payload
                output = io.StringIO()
                with contextlib.redirect_stdout(output):
                    client.display(file_message)
                    assert file_url in output.getvalue()
                    listener = threading.Thread(target=client.receive, daemon=True)
                    listener.start()
                    client.send_text("Python 修改剪贴板\n第二行", "clipboard")
                    wait_for(lambda: client.clipboard() == "Python 修改剪贴板\n第二行")
                    send_owner({"kind": "text", "source": "user", "text": "普通聊天"})
                    wait_for(
                        lambda: any(
                            message.get("text") == "普通聊天"
                            for message in client.history()
                        )
                    )
                    assert client.clipboard() == "Python 修改剪贴板\n第二行"
                    send_owner(
                        {
                            "kind": "text",
                            "source": "clipboard",
                            "text": "模拟 Tauri 复制",
                        }
                    )
                    wait_for(lambda: client.clipboard() == "模拟 Tauri 复制")
                    try:
                        client.send_text("   ", "clipboard")
                        raise AssertionError("空白剪贴板应被拒绝")
                    except ValueError:
                        pass
                # Verify the runnable CLI, including history and authenticated URL output.
                cli = subprocess.run(
                    [
                        os.sys.executable,
                        str(Path(__file__).with_name("room_client.py")),
                        room_id,
                        "--api",
                        origin,
                    ],
                    input=f"/get\n/history\n/files\n/download {file_message['id']}\n/quit\n",
                    text=True,
                    capture_output=True,
                    timeout=15,
                    env=env,
                    check=False,
                )
                assert cli.returncode == 0, cli.stderr
                assert "模拟 Tauri 复制" in cli.stdout and file_url in cli.stdout
                assert "Authorization: Bearer" in cli.stdout
                print(
                    "PASS: 加入房间、200+ 条历史分页、读取/修改剪贴板、实时聊天、文件 URL/授权下载、交互命令。"
                )
            except Exception:
                log.flush()
                log.seek(0)
                print(log.read()[-4000:])
                raise
            finally:
                if client:
                    client.close()
                if owner:
                    owner.close()
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--server", required=True, help="已编译的本地 ClipBox Go 服务可执行文件"
    )
    run(parser.parse_args().server)
