from flask import Blueprint, jsonify
from app.extensions import redis_client
import json
import requests
import time
import threading

likemusic_bp = Blueprint('likemusic', __name__)

last_updated = -1


def get_likemusic_data():
    headers = {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0 MeT-Box/1.0.0',
    }

    response = requests.get('https://met6.top/homepage_music.php', headers=headers)

    data = response.json()

    if data['status'] != "success":
        return []
    music_data = []
    for music in data['data']:
        music_data.append({
            "mid": music['songmid'],
            "name": music['title'],
            "artist": music['author'],
            "url": music['url'],
            "cover": music['pic'],
        })
    return music_data


def update_redis_periodically(interval=2*60*60):
    global redis_client
    global last_updated

    while True:
        try:
            current_time = time.time()
            if last_updated == -1 or (current_time - last_updated) > interval:
                last_updated = current_time
                likemusic_data = get_likemusic_data()
                redis_client.set("LIKEMUSIC_DATA", json.dumps(likemusic_data))
        except:
            pass
        time.sleep(interval)


def get_music_likes_cache():
    global redis_client
    global last_updated

    for i in range(5):
        if last_updated == -1 or (time.time() - last_updated) > 2*60*60:
            last_updated = time.time()
            likemusic_data = get_likemusic_data()
            redis_client.set("LIKEMUSIC_DATA", json.dumps(likemusic_data))
        else:
            try:
                redis_data = redis_client.get("LIKEMUSIC_DATA")
                if redis_data is None:
                    likemusic_data = []
                    last_updated == -1
                else:
                    likemusic_data = json.loads(redis_data)
                    return likemusic_data
            except:
                likemusic_data = []
                last_updated == -1
    return likemusic_data


@likemusic_bp.route("/")
def get_music_likes():
    global redis_client
    global last_updated

    return jsonify(get_music_likes_cache())


thread = threading.Thread(target=update_redis_periodically)
thread.daemon = True
thread.start()
