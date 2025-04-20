from flask import Flask
from .extensions import redis_client
from .routes import register_blueprints


def create_app():
    app = Flask(__name__)
    app.config.from_object('config.Config')
    app.json.sort_keys = False
    app.url_map.strict_slashes = False

    redis_client.init_app(app)
    register_blueprints(app)

    return app
