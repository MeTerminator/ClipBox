from flask import Blueprint, jsonify, render_template_string
from app.extensions import redis_client
import json

fileshare_bp = Blueprint('fileshare', __name__)


@fileshare_bp.route("/clash/<file>")
def get_clash(file):
    url = redis_client.get("FILESHARE_SECTION_A")
    if url:
        url = json.loads(url)
        if file == "exe":
            return jump_without_referer(url["clash_exe"])
        elif file == "apk":
            return jump_without_referer(url["clash_apk"])
    return jsonify({"error": "Not found"}), 404


def jump_without_referer(target_url):
    html = f"""
    <!DOCTYPE html>
    <html>
    <head>
        <meta name="referrer" content="no-referrer">
        <meta http-equiv="refresh" content="0;url={target_url}">
        <script>
            location.replace("{target_url}");
        </script>
    </head>
    <body>
        <p>Redirecting...</p>
    </body>
    </html>
    """
    return render_template_string(html)
