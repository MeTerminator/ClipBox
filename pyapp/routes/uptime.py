from flask import Blueprint, jsonify, make_response
from pyapp.extensions import redis_client
import json
import requests
import time

uptime_bp = Blueprint('uptime', __name__)


NODES_NAME = {
    "k1": "New York",
    "k4": "London",
    "k6": "Singapore",
    "k10": "Tokyo",
}

last_updated = -1


def split_list(data):
    last_two = data[-2:]
    remaining = data[:-2]
    groups = [remaining[i:i+3] for i in range(0, len(remaining), 3)]
    return groups, last_two


def get_uptime_data():
    # Request uptime data
    headers = {
        'Origin': 'https://uptime.met6.top',
        'Referer': 'https://uptime.met6.top/report/uptime/77cd610a8bb67f98f485b02743f2c253/',
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0',
        'X-Requested-With': 'XMLHttpRequest',
    }

    data = {
        't': '77cd610a8bb67f98f485b02743f2c253',
        'u': '1',
        'v': '1',
    }

    response = requests.post('https://uptime.met6.top/u_rb.php', headers=headers, data=data)
    response_text = response.text
    # Process uptime data
    uptime_data = {
        "name": "MeT-Website",
        "url": "https://met6.top/",
        "status": "offline",
        "last_updated": -1,
        "last_stat_change": "",
        "average_response_time": -1,
        "nodes": {},
    }

    response_text = response_text.replace('<script>if(!document.hidden){', '')
    response_text = response_text.replace('}</script>', '')
    response_text = response_text.split(';')[:-1]

    node_tags, overall_tags = split_list(response_text)
    for node_tag in node_tags:
        tag1 = node_tag[0].split("').removeClass('fa-arrow-up').removeClass('fa-arrow-down').addClass('")
        node_id = tag1[0].split('_')[1]
        if node_id in NODES_NAME:
            node_name = NODES_NAME[node_id]
        else:
            node_name = "Unknown"
        node_status = 'online' if 'fa-arrow-up' in tag1[1] else 'offline'
        if node_status == 'online':
            uptime_data['status'] = 'online'
        node_latency = int(node_tag[1].split(".text('")[1].replace("ms')", ""))
        node_last_updated = int(node_tag[2].split(".attr('data-timez', '")[1].replace("')", ""))
        uptime_data['nodes'][node_id] = {
            "name": node_name,
            "status": node_status,
            "latency": node_latency,
            "last_updated": node_last_updated,
        }

    uptime_data['last_stat_change'] = (overall_tags[0].split("Since ")[1].replace(" ago')", ""))
    uptime_data['last_updated'] = int(overall_tags[1].split(".attr('data-timez', '")[1].replace("')", ""))
    addup_latency = 0
    for node_id in uptime_data['nodes']:
        addup_latency += uptime_data['nodes'][node_id]['latency']
    uptime_data['average_response_time'] = int(addup_latency / len(uptime_data['nodes']))

    return uptime_data


@uptime_bp.route("/details")
def get_uptime_details():
    global redis_client
    global last_updated
    if last_updated == -1 or (time.time() - last_updated) > 60:
        last_updated = time.time()
        uptime_data = get_uptime_data()
        redis_client.set("UPTIME_DATA", json.dumps(uptime_data))
    else:
        try:
            uptime_data = json.loads(redis_client.get("UPTIME_DATA"))
        except:
            uptime_data = {}
    return jsonify(uptime_data)


@uptime_bp.route("/")
def get_uptime():
    global redis_client
    global last_updated

    if last_updated == -1 or (time.time() - last_updated) > 60:
        last_updated = time.time()
        uptime_data = get_uptime_data()
        redis_client.set("UPTIME_DATA", json.dumps(uptime_data))
    else:
        try:
            uptime_data = json.loads(redis_client.get("UPTIME_DATA"))
        except:
            uptime_data = {}

    response = make_response(jsonify({
        "status": uptime_data.get("status"),
        "last_updated": uptime_data.get("last_updated")
    }))

    response.headers["Uptime-Status"] = str(uptime_data.get("status")).capitalize()

    return response
