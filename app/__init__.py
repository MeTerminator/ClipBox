from flask import Flask, send_from_directory, redirect, request
import os
from .extensions import redis_client
from .routes import register_blueprints


def create_app():
    app = Flask(__name__)
    app.config.from_object('app.config.Config')
    app.json.sort_keys = False
    app.url_map.strict_slashes = False

    redis_client.init_app(app)
    register_blueprints(app)

    public_dir = os.path.join(os.path.dirname(__file__), "../public")

    # 返回首页 index.html
    @app.route("/", endpoint="index")
    def serve_index():
        return send_from_directory(public_dir, "index.html")

    # 返回 public 目录下的其他静态文件或目录
    @app.route("/<path:path>", endpoint="static_files")
    def serve_static_file(path):
        full_path = os.path.join(public_dir, path)

        # 如果是目录，但 URL 没有以 `/` 结尾，重定向到带 `/` 的 URL
        if os.path.isdir(full_path) and not request.path.endswith('/'):
            return redirect(request.path + '/')

        # 如果是目录，返回 index.html
        if os.path.isdir(full_path):
            return send_from_directory(public_dir, os.path.join(path, "index.html"))

        # 如果是文件并且存在，返回文件
        if os.path.exists(full_path):
            return send_from_directory(public_dir, path)

        # 如果文件不存在，尝试返回目录中的 index.html
        if os.path.exists(os.path.join(public_dir, path, "index.html")):
            return send_from_directory(public_dir, os.path.join(path, "index.html"))

        # 最后兜底返回根目录 index.html（防止出错）
        return send_from_directory(public_dir, "index.html")

    return app
