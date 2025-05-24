from flask import redirect, Blueprint, request, jsonify, make_response
from app.extensions import redis_client
import json
import uuid
import time
import threading
import re

clip_bp = Blueprint('clip', __name__)


URL_REGEX = re.compile(
    r'^(https?://)'
    r'((([A-Za-z0-9-]+\.)+[A-Za-z]{2,})|'  # 域名
    r'localhost|'                         # 本地地址
    r'(\d{1,3}\.){3}\d{1,3})'             # 或者 IP 地址
    r'(:\d+)?'                            # 可选端口
    r'(/[\w\-./?%&=]*)?$'                 # 路径
)


@clip_bp.route("/create", methods=["POST"])
def create_clip():
    try:
        count = int(request.form.get("count", 1))
        expire = int(request.form.get("expire", 3600))
    except ValueError:
        return jsonify({"error": "Invalid count or expire"}), 400

    is_link = request.form.get("link", "no").lower() == "yes"
    content = request.form.get("content", "").strip()

    if not content:
        return jsonify({"error": "Missing content"}), 400

    if is_link and not URL_REGEX.match(content):
        return jsonify({"error": "Content must be a valid URL when link=yes"}), 400

    content_type = "link" if is_link else "text/plain"

    code = str(uuid.uuid4())[:8]
    created_at = int(time.time())

    content_key = f"CLIP_DATA_{code}"
    meta_key = f"CLIP_META_{code}"

    redis_client.set(content_key, content)
    redis_client.set(meta_key, json.dumps({
        "count": count,
        "expire": expire,
        "created_at": created_at,
        "content_type": content_type
    }))

    return jsonify({"code": code})


@clip_bp.route("/get/<code>", methods=["GET"])
def get_clip(code):
    content_key = f"CLIP_DATA_{code}"
    meta_key = f"CLIP_META_{code}"

    content = redis_client.get(content_key)
    meta_raw = redis_client.get(meta_key)

    if not content or not meta_raw:
        return "Not found", 404

    # 解码meta信息
    meta = json.loads(meta_raw)

    # 检查是否过期
    current_time = int(time.time())
    if current_time - meta["created_at"] > meta["expire"]:
        redis_client.delete(content_key)
        redis_client.delete(meta_key)
        return "Not found", 404

    # 如果是链接类型，检查count是否为0
    if meta.get("content_type") == "link":
        if meta["count"] <= 0:
            redis_client.delete(content_key)
            redis_client.delete(meta_key)
            return "Not found", 404

        # 减少访问次数
        meta["count"] -= 1
        redis_client.set(meta_key, json.dumps(meta))

        # 返回302重定向
        content = content.decode('utf-8')  # 确保是字符串
        return redirect(content, code=302)

    # 如果是文本类型，减少访问次数并返回内容
    meta["count"] -= 1
    if meta["count"] <= 0:
        redis_client.delete(content_key)
        redis_client.delete(meta_key)
    else:
        redis_client.set(meta_key, json.dumps(meta))

    # 创建响应并设置Content-Type为UTF-8
    resp = make_response(content)
    resp.headers["Content-Type"] = meta.get("content_type", "text/plain; charset=utf-8")
    return resp


def cleanup_expired():
    while True:
        time.sleep(600)  # 每 10 分钟清理一次
        now = int(time.time())
        for key in redis_client.scan_iter(match="CLIP_META_*"):
            try:
                code = key.replace("CLIP_META_", "")
                meta_raw = redis_client.get(key)
                if not meta_raw:
                    continue
                meta = json.loads(meta_raw)
                created = meta.get("created_at", 0)
                expire = meta.get("expire", 0)
                if now - created > expire:
                    redis_client.delete(f"CLIP_META_{code}")
                    redis_client.delete(f"CLIP_DATA_{code}")
            except:
                continue


# 启动后台清理线程
threading.Thread(target=cleanup_expired, daemon=True).start()
