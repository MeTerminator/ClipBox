from flask import Flask, jsonify, request

app = Flask(__name__)

# 默认考试时间
exams = [
    {
        "name": "语文",
        "start_at": "2025-06-07 09:00:00",
        "duration_hour": 2.5
    },
    {
        "name": "数学",
        "start_at": "2025-06-07 15:00:00",
        "duration_hour": 2
    },
    {
        "name": "物理历史",
        "start_at": "2025-06-08 09:00:00",
        "duration_hour": 1.25
    },
    {
        "name": "外语",
        "start_at": "2025-06-08 15:00:00",
        "duration_hour": 2
    },
    {
        "name": "化学",
        "start_at": "2025-06-09 08:30:00",
        "duration_hour": 1.25
    },
    {
        "name": "地理",
        "start_at": "2025-06-09 11:00:00",
        "duration_hour": 1.25
    },
    {
        "name": "思想政治",
        "start_at": "2025-06-09 14:30:00",
        "duration_hour": 1.25
    },
    {
        "name": "生物",
        "start_at": "2025-06-09 17:00:00",
        "duration_hour": 1.25
    },
]

exams = []


@app.route("/api/exams", methods=["GET"])
def get_exams():
    return jsonify(exams)


@app.route("/api/set_exams", methods=["POST"])
def set_exams():
    global exams
    if not request.is_json:
        return jsonify({"error": "Invalid JSON"}), 400

    data = request.get_json()
    if not isinstance(data, list):
        return jsonify({"error": "Expected a list of exams"}), 400

    for exam in data:
        if not all(key in exam for key in ("name", "start_at", "duration_hour")):
            return jsonify({"error": "Each exam must have name, start_at, and duration_hour"}), 400

    exams = data  # 更新考试信息
    return jsonify({"message": "Exams updated successfully"})


if __name__ == "__main__":
    app.run(port=5328, debug=False)
