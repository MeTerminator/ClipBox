from flask import Flask, jsonify

app = Flask(__name__)


@app.route("/api/exams")
def get_exams():
    exams = [
        {
            "name": "A",
            "start_at": "2025-03-08 19:14:00",
            "duration_hour": 2.5
        },
        {
            "name": "B",
            "start_at": "2025-03-08 20:15:00",
            "duration_hour": 2
        },
        {
            "name": "C",
            "start_at": "2025-03-08 23:00:00",
            "duration_hour": 2
        }
    ]
    return jsonify(exams)


if __name__ == "__main__":
    app.run(port=5328, debug=True)
