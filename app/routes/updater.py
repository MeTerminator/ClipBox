from flask import Blueprint, request, jsonify
from app.extensions import redis_client
import json

updater_bp = Blueprint('updater', __name__)

default_config = {
    "version": "0",
    "interval": 3600,
    "payload": []
}


@updater_bp.route("/", methods=["GET", "POST"])
def get_updater_config():
    global redis_client
    try:
        config = json.loads(redis_client.get("UPDATER_CONFIG"))
    except:
        redis_client.set("UPDATER_CONFIG", json.dumps(default_config))
        config = default_config
    return jsonify(config)
