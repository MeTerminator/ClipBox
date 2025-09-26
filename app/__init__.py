from flask import Flask, send_from_directory, request
from flask_cors import CORS
from app.routes import register_blueprints
from app.database import db, init_database
import os

# https://github.com/MeTerminator/ClipBox

def create_app():
    app = Flask(__name__)
    app.config.from_object('app.config.Config')
    app.json.sort_keys = False
    app.url_map.strict_slashes = False

    CORS(app)

    # 初始化数据库
    db.init_app(app)

    # 创建数据库表
    init_database(app)

    # 注册蓝图
    register_blueprints(app)

    # 静态资源根目录（使用绝对路径，避免工作目录差异导致 404）
    project_root = os.path.dirname(os.path.dirname(__file__))
    www_root = os.path.join(project_root, 'www')

    # 显式根路由，直接返回首页
    @app.route('/')
    def _index_page():
        try:
            return send_from_directory(www_root, 'index.html')
        except Exception:
            # 若首页不存在，返回 404
            return ("index.html not found", 404)

    # 静态资源回退：当未命中任何蓝图（返回 404）时，尝试从 www/ 目录返回静态文件
    @app.errorhandler(404)
    def _serve_www_on_404(e):
        # 对 /api/* 保持 API 语义，直接返回原始 404
        req_path = request.path.lstrip('/')
        if req_path.startswith('api/'):
            return e

        # 1) 根目录与精确文件命中
        try:
            return send_from_directory(www_root, req_path if req_path else 'index.html')
        except Exception:
            pass

        # 2) 目录命中 index.html（支持 /dir 与 /dir/ 两种形式）
        if req_path:
            try:
                return send_from_directory(www_root, req_path.rstrip('/') + '/index.html')
            except Exception:
                pass

        # 3) 根目录兜底 index.html
        try:
            return send_from_directory(www_root, 'index.html')
        except Exception:
            return e

    return app
