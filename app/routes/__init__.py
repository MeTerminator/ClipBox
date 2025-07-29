from .main import main_bp
from .ntptime import ntptime_bp
from .clip import clip_bp


def register_blueprints(app):
    app.register_blueprint(main_bp, url_prefix='/api')
    app.register_blueprint(ntptime_bp, url_prefix='/api/ntptime')
    app.register_blueprint(clip_bp, url_prefix='/api/clip')
