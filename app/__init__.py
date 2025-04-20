from flask import Flask, send_from_directory
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

    # 返回首页 index.html
    @app.route("/", endpoint="index")
    def serve_index():
        return send_from_directory(os.path.join(os.path.dirname(__file__), "../public"), "index.html")

    # 返回 public 目录下的其他静态文件
    @app.route("/<path:path>", endpoint="static_files")
    def serve_static_file(path):
        return send_from_directory(os.path.join(os.path.dirname(__file__), "../public"), path)

    return app
