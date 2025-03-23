from flask import Flask, jsonify, request
import json
import redis
import os

# MeTerminator
# Homepage: https://met6.top/
# QQ: 3532095196

# 如果你看到了这个，那么你已经成功的发现了更新考试信息的API
# 太懒没做鉴权，欢迎通过 QQ 联系我，和我分享你的发现 (doge)
# 你不会通过这个更新考试的 API 来乱搞的对吧 (doge)
# 不，我是不会把我的 Redis 凭据写进代码里的 (doge)

r = redis.Redis.from_url(os.getenv("REDIS_URL"))

app = Flask(__name__)


@app.route("/api/exams", methods=["GET"])
def get_exams():
    global r
    try:
        exams = json.loads(r.get("exams"))
    except:
        exams = []
    return jsonify(exams)


@app.route("/api/set_exams", methods=["POST"])
def set_exams():
    global r
    if not request.is_json:
        return jsonify({"error": "Invalid JSON"}), 400

    data = request.get_json()
    if not isinstance(data, list):
        return jsonify({"error": "Expected a list of exams"}), 400

    for exam in data:
        if not all(key in exam for key in ("name", "start_at", "duration_hour")):
            return jsonify({"error": "Each exam must have name, start_at, and duration_hour"}), 400

    exams = data  # 更新考试信息
    r.set("exams", json.dumps(exams))  # 更新redis中的考试信息
    return jsonify({"message": "Exams updated successfully"})


if __name__ == "__main__":
    app.run(port=5328, debug=False)
