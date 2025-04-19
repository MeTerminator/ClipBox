from flask import Blueprint, jsonify
import ntplib


def get_ntp_time():
    host = "ntp.aliyun.com"
    client = ntplib.NTPClient()
    try:
        response = client.request(host, version=3)
        return int(response.tx_time * 1000)
    except Exception as e:
        print("NTP error:", e)
        return None


ntptime_bp = Blueprint('ntptime', __name__)


@ntptime_bp.route("/")
def get_exams():

    return jsonify({"timestamp": get_ntp_time()})
