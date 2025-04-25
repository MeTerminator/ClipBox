from .main import main_bp
from .uptime import uptime_bp
from .exams import exams_bp
from .ntptime import ntptime_bp
from .clip import clip_bp


def register_blueprints(app):
    app.register_blueprint(main_bp, url_prefix='/api')
    app.register_blueprint(uptime_bp, url_prefix='/api/uptime')
    app.register_blueprint(exams_bp, url_prefix='/api/exams')
    app.register_blueprint(ntptime_bp, url_prefix='/api/ntptime')
    app.register_blueprint(clip_bp, url_prefix='/api/clip')
