from flask import Blueprint, jsonify, make_response, redirect
from app.extensions import redis_client
import json
import requests
import time
import threading

mets_bp = Blueprint('mets', __name__)

NODES_NAME = {
    "k1": "New York",
    "k4": "London",
    "k6": "Singapore",
    "k10": "Tokyo",
}

last_uptime_updated = -1
last_music_updated = -1
last_metsstapi_updated = -1


@mets_bp.after_request
def add_cors_headers(response):
    response.headers['Access-Control-Allow-Origin'] = '*'
    return response

# ------------------- 公用函数 -------------------


def split_list(data):
    last_two = data[-2:]
    remaining = data[:-2]
    groups = [remaining[i:i+3] for i in range(0, len(remaining), 3)]
    return groups, last_two

# ------------------- 获取数据 -------------------


def get_uptime_data():
    uptime_data = {
        "name": "MeT-Website",
        "url": "https://met6.top/",
        "status": "offline",
        "last_updated": -1,
        "last_stat_change": "",
        "average_response_time": -1,
        "nodes": {},
    }
    headers = {
        'Origin': 'https://uptime.met6.top',
        'Referer': 'https://uptime.met6.top/report/uptime/77cd610a8bb67f98f485b02743f2c253/',
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',
        'X-Requested-With': 'XMLHttpRequest',
    }
    data = {
        't': '77cd610a8bb67f98f485b02743f2c253',
        'u': '1',
        'v': '1',
    }

    for _ in range(5):
        r = requests.post('https://uptime.met6.top/u_rb.php', headers=headers, data=data)
        text = r.text
        if '<script>location.reload();</script>' in text:
            data['u'] = '0' if data['u'] == '1' else '1'
            continue
        break

    text = text.replace('<script>if(!document.hidden){', '').replace('}</script>', '')
    tags = text.split(';')[:-1]

    node_tags, overall_tags = split_list(tags)
    for tag in node_tags:
        t1 = tag[0].split("').removeClass('fa-arrow-up').removeClass('fa-arrow-down').addClass('")
        node_id = t1[0].split('_')[1]
        node_name = NODES_NAME.get(node_id, "Unknown")
        status = 'online' if 'fa-arrow-up' in t1[1] else 'offline'
        if status == 'online':
            uptime_data['status'] = 'online'
        latency = int(tag[1].split(".text('")[1].replace("ms')", ""))
        ts = int(tag[2].split(".attr('data-timez', '")[1].replace("')", ""))
        uptime_data['nodes'][node_id] = {
            "name": node_name,
            "status": status,
            "latency": latency,
            "last_updated": ts,
        }

    uptime_data['last_stat_change'] = overall_tags[0].split("Since ")[1].replace(" ago')", "").replace("')", "")
    uptime_data['last_updated'] = int(overall_tags[1].split(".attr('data-timez', '")[1].replace("')", ""))
    latencies = [n["latency"] for n in uptime_data['nodes'].values()]
    uptime_data['average_response_time'] = int(sum(latencies) / len(latencies)) if latencies else -1
    return uptime_data


def get_likemusic_data():
    headers = {'User-Agent': 'Mozilla/5.0 MeT-Box/1.0.0'}
    r = requests.get('https://met6.top/homepage_music.php', headers=headers)
    data = r.json()
    if data['status'] != "success":
        return []
    return [{
        "mid": m['songmid'],
        "name": m['title'],
        "artist": m['author'],
        "url": m['url'],
        "cover": m['pic'],
    } for m in data['data']]


def get_metsstapi_data():
    r = requests.get("https://met6.top/metsstapi.php", timeout=5)
    return r.json()

# ------------------- 更新函数（合并） -------------------


def update_all_cache():
    global last_uptime_updated, last_music_updated, last_metsstapi_updated
    try:
        redis_client.set("UPTIME_DATA", json.dumps(get_uptime_data()))
        last_uptime_updated = time.time()
    except:
        pass
    try:
        redis_client.set("LIKEMUSIC_DATA", json.dumps(get_likemusic_data()))
        last_music_updated = time.time()
    except:
        pass
    try:
        redis_client.set("METSSTAPI_DATA", json.dumps(get_metsstapi_data()))
        last_metsstapi_updated = time.time()
    except:
        pass


def async_update():
    threading.Thread(target=update_all_cache, daemon=True).start()

# ------------------- 路由 -------------------


@mets_bp.route("/")
def redirect_to_metwebsite():
    return redirect("https://met6.top/")


@mets_bp.route("/uptime/")
def get_uptime():
    if time.time() - last_uptime_updated > 60:
        async_update()
    try:
        data = redis_client.get("UPTIME_DATA")
        d = json.loads(data) if data else {}
    except:
        d = {}
    return jsonify({
        "status": d.get("status"),
        "last_updated": d.get("last_updated")
    })


@mets_bp.route("/uptime/details")
def get_uptime_details():
    if time.time() - last_uptime_updated > 60:
        async_update()
    try:
        data = redis_client.get("UPTIME_DATA")
        return jsonify(json.loads(data)) if data else jsonify({})
    except:
        return jsonify({})


@mets_bp.route("/likemusic/")
def get_music_likes():
    if time.time() - last_music_updated > 2 * 60 * 60:
        async_update()
    try:
        data = redis_client.get("LIKEMUSIC_DATA")
        return jsonify(json.loads(data)) if data else jsonify([])
    except:
        return jsonify([])


@mets_bp.route("/metsstapi")
def get_metsstapi():
    if time.time() - last_metsstapi_updated > 60:
        async_update()
    try:
        data = redis_client.get("METSSTAPI_DATA")
        return jsonify(json.loads(data)) if data else jsonify({})
    except:
        return jsonify({})


@mets_bp.route("/update")
def manual_update():
    async_update()
    return jsonify({"status": "updated"})
