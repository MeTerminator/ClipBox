from .main import main_bp
from .mets import mets_bp
from .exams import exams_bp
from .ntptime import ntptime_bp
from .clip import clip_bp
from .updater import updater_bp


def register_blueprints(app):
    app.register_blueprint(main_bp, url_prefix='/api')
    app.register_blueprint(mets_bp, url_prefix='/api/mets')
    app.register_blueprint(exams_bp, url_prefix='/api/exams')
    app.register_blueprint(ntptime_bp, url_prefix='/api/ntptime')
    app.register_blueprint(clip_bp, url_prefix='/api/clip')
    app.register_blueprint(updater_bp, url_prefix='/api/updater')
