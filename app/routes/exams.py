from flask import Blueprint, request, jsonify
from app.extensions import redis_client
import json

exams_bp = Blueprint('exams', __name__)


@exams_bp.route("/")
def get_exams():
    global redis_client
    try:
        exams = json.loads(redis_client.get("EXAMS_DATA"))
    except:
        exams = []
    return jsonify(exams)


@exams_bp.route("/set", methods=["POST"])
def set_exams():
    global redis_client
    if not request.is_json:
        return jsonify({"error": "Invalid JSON"}), 400

    data = request.get_json()
    if not isinstance(data, list):
        return jsonify({"error": "Expected a list of exams"}), 400

    for exam in data:
        if not all(key in exam for key in ("name", "start_at", "duration_hour")):
            return jsonify({"error": "Each exam must have name, start_at, and duration_hour"}), 400

    new_exams = []
    for exam in data:
        new_exams.append({
            "name": exam["name"],
            "start_at": exam["start_at"],
            "duration_hour": exam["duration_hour"]
        })
    exams = new_exams

    redis_client.set("EXAMS_DATA", json.dumps(exams))
    return jsonify({"message": "Exams updated successfully"})
