from flask import redirect, Blueprint, request, jsonify, make_response, current_app
from app.database import db
from app.database import Clip, cleanup_expired_clips
import random
import validators
import hashlib
import os
import mimetypes
import glob
from datetime import datetime
from app.utils import get_real_ip
import threading
from urllib.parse import quote

clip_bp = Blueprint('clip', __name__)

# https://github.com/MeTerminator/ClipBox

@clip_bp.route("/create", methods=["POST"])
def create_clip():
    try:
        count = int(request.form.get("count", 1))
        expire = int(request.form.get("expire", 3600))
    except ValueError:
        return jsonify({"error": "Invalid count or expire"}), 400

    is_link = request.form.get("link", "no").lower() == "yes"
    content = request.form.get("content", "").strip()

    if not content:
        return jsonify({"error": "Missing content"}), 400

    # 链接校验与长度限制
    if is_link and not validators.url(content):
        return jsonify({"error": "Content must be a valid URL when link=yes"}), 400
    if is_link and len(content) > int(current_app.config.get('MAX_LINK_LENGTH', 2048)):
        return jsonify({"error": "Link is too long"}), 400

    # 文本大小限制（按UTF-8字节数计算）
    if not is_link:
        max_text = int(current_app.config.get('MAX_TEXT_SIZE', 5 * 1024 * 1024))
        if len(content.encode('utf-8')) > max_text:
            return jsonify({"error": "Text content too large"}), 400

    content_type = "link" if is_link else "text/plain"
    code = str(random.randint(10000, 99999))

    # 确保代码唯一性
    while Clip.query.filter_by(code=code).first():
        code = str(random.randint(10000, 99999))

    # 记录客户端IP
    client_ip = get_real_ip(request)

    # 创建新的剪贴板记录
    clip = Clip(
        code=code,
        content_type=content_type,
        content=content,
        client_ip=client_ip,
        access_count=count,
        max_count=count,
        expire_seconds=expire
    )

    try:
        db.session.add(clip)
        db.session.commit()
        return jsonify({"code": code})
    except Exception as e:
        db.session.rollback()
        return jsonify({"error": "Database error"}), 500


@clip_bp.route("/get/<code>", methods=["GET"])
def get_clip(code):
    clip = Clip.query.filter_by(code=code).first()
    if not clip:
        return "Not found", 404

    # 检查是否过期
    if clip.is_expired():
        # 删除过期记录
        if clip.content_type == 'file' and clip.file_path:
            try:
                other_refs = 0
                if clip.file_hash:
                    other_refs = Clip.query.filter(
                        Clip.file_hash == clip.file_hash,
                        Clip.id != clip.id
                    ).count()
                if other_refs == 0 and os.path.exists(clip.file_path):
                    os.remove(clip.file_path)
            except OSError:
                pass

        db.session.delete(clip)
        db.session.commit()
        return "Not found", 404

    # 如果是链接类型，检查count是否为0
    if clip.content_type == "link":
        if clip.access_count <= 0:
            db.session.delete(clip)
            db.session.commit()
            return "Not found", 404

        # 减少访问次数
        clip.access_count -= 1
        if clip.access_count <= 0:
            db.session.delete(clip)
        db.session.commit()

        # 返回302重定向
        return redirect(clip.content, code=302)

    # 如果是文件类型，处理文件下载
    if clip.content_type == "file":
        # 检查文件是否存在
        if not clip.file_path or not os.path.exists(clip.file_path):
            db.session.delete(clip)
            db.session.commit()
            return "File not found on disk", 404

        # 若哈希尚未计算完成，暂不开放下载
        if not clip.file_hash:
            return jsonify({"code": code, "status": "processing", "message": "File is processing, please try again later"}), 425

        # 减少访问次数
        clip.access_count -= 1
        if clip.access_count <= 0:
            # 删除文件和记录（若无其他引用）
            try:
                other_refs = 0
                if clip.file_hash:
                    other_refs = Clip.query.filter(
                        Clip.file_hash == clip.file_hash,
                        Clip.id != clip.id
                    ).count()
                if other_refs == 0 and os.path.exists(clip.file_path):
                    os.remove(clip.file_path)
            except OSError:
                pass
            db.session.delete(clip)
            db.session.commit()
        else:
            db.session.commit()

        # 返回文件，支持断点续传
        try:
            file_size = clip.file_size or 0
            mime_type = clip.mime_type or 'application/octet-stream'

            # 判断是否可以在浏览器中直接查看
            # viewable_types = [
            #     'text/', 'image/', 'video/', 'audio/', 'application/pdf',
            #     'application/json', 'application/xml', 'application/javascript'
            # ]
            viewable_types = [
                'text/', 'application/json', 'application/xml', 'application/javascript'
            ]

            is_viewable = any(mime_type.startswith(vtype) for vtype in viewable_types)

            # 检查是否有Range请求头（断点续传）
            range_header = request.headers.get('Range')

            if range_header:
                # 解析Range头
                range_match = range_header.replace('bytes=', '').split('-')
                start = int(range_match[0]) if range_match[0] else 0
                end = int(range_match[1]) if range_match[1] else file_size - 1

                # 确保范围有效
                if start >= file_size or end >= file_size or start > end:
                    response = make_response("Requested Range Not Satisfiable", 416)
                    response.headers['Content-Range'] = f'bytes */{file_size}'
                    return response

                # 读取指定范围的文件内容
                with open(clip.file_path, 'rb') as f:
                    f.seek(start)
                    chunk_size = end - start + 1
                    file_content = f.read(chunk_size)

                response = make_response(file_content)
                response.status_code = 206  # Partial Content
                response.headers['Content-Range'] = f'bytes {start}-{end}/{file_size}'
                response.headers['Accept-Ranges'] = 'bytes'
                response.headers['Content-Length'] = str(chunk_size)
            else:
                # 完整文件下载
                with open(clip.file_path, 'rb') as f:
                    file_content = f.read()

                response = make_response(file_content)
                response.headers['Accept-Ranges'] = 'bytes'
                response.headers['Content-Length'] = str(file_size)

            # 处理非ASCII文件名（包括中文）：同时设置 filename 和 RFC 5987 的 filename*
            disp_type = 'inline' if is_viewable else 'attachment'
            fn = clip.filename or "file"
            safe_fn_star = quote(fn, encoding='utf-8')
            # 注意：某些浏览器对 filename 的 UTF-8 兼容良好，这里直接放原始名作回退
            response.headers['Content-Disposition'] = f"{disp_type}; filename=\"{fn}\"; filename*=UTF-8''{safe_fn_star}"

            response.headers['Content-Type'] = mime_type
            return response
        except Exception as e:
            return f"Error reading file: {str(e)}", 500

    # 如果是文本类型，减少访问次数并返回内容
    clip.access_count -= 1
    if clip.access_count <= 0:
        db.session.delete(clip)
    db.session.commit()

    # 创建响应并设置Content-Type为UTF-8
    resp = make_response(clip.content)
    content_type = clip.content_type
    if content_type == "text/plain":
        resp.headers["Content-Type"] = "text/plain; charset=utf-8"
    else:
        resp.headers["Content-Type"] = f"{content_type}; charset=utf-8"
    return resp


@clip_bp.route("/upload", methods=["POST"])
def upload_file():
    try:
        count = int(request.form.get("count", 1000))
        expire = int(request.form.get("expire", 604800))
    except ValueError:
        return jsonify({"error": "Invalid count or expire"}), 400

    if 'file' not in request.files:
        return jsonify({"error": "No file provided"}), 400

    file = request.files['file']
    if file.filename == '':
        return jsonify({"error": "No file selected"}), 400

    # 创建基于日期的目录结构
    current_date = datetime.now().strftime("%Y-%m-%d")
    upload_dir = os.path.join("data", current_date)
    os.makedirs(upload_dir, exist_ok=True)

    # 生成唯一代码
    code = str(random.randint(10000, 99999))
    while Clip.query.filter_by(code=code).first():
        code = str(random.randint(10000, 99999))

    # 原始文件名（保留中文供展示与下载头），用于磁盘路径的文件名仅做最小化清理
    original_filename = file.filename or ''
    if not original_filename:
        original_filename = f"file_{code}"

    # 读取客户端计算的SHA256（可选）
    client_hash = request.form.get('client_sha256', '').strip().lower()
    def _is_valid_sha256(h: str) -> bool:
        return len(h) == 64 and all(c in '0123456789abcdef' for c in h)
    client_hash_valid = _is_valid_sha256(client_hash)

    def sanitize_filename(name: str) -> str:
        # 去除路径注入，并替换 Windows 非法字符，但保留中文与其他Unicode
        base = os.path.basename(name)
        # Windows 非法字符 <>:"/\|?* 以及控制字符
        illegal = '<>:"/\\|?*'
        cleaned = ''.join('_' if (ch in illegal or ord(ch) < 32) else ch for ch in base)
        # 避免空名
        return cleaned or f"file_{code}"

    safe_name_for_path = sanitize_filename(original_filename)
    file_path = os.path.join(upload_dir, f"{code}_{safe_name_for_path}")

    # 保存文件（流式），仅统计大小；哈希将由后台线程验证
    file_size = 0
    too_large = False
    max_file_size = int(current_app.config.get('MAX_UPLOAD_FILE_SIZE', 500 * 1024 * 1024))

    try:
        with open(file_path, 'wb') as f:
            while True:
                chunk = file.stream.read(8192)  # 8KB chunks
                if not chunk:
                    break
                f.write(chunk)
                file_size += len(chunk)
                if file_size > max_file_size:
                    too_large = True
                    break
    except Exception as e:
        # 如果保存失败，删除部分文件
        if os.path.exists(file_path):
            os.remove(file_path)
        return jsonify({"error": f"Failed to save file: {str(e)}"}), 500

    if too_large:
        if os.path.exists(file_path):
            os.remove(file_path)
        return jsonify({"error": "File too large"}), 413

    # 检测文件MIME类型
    mime_type, _ = mimetypes.guess_type(original_filename)
    if not mime_type:
        mime_type = "application/octet-stream"

    client_ip = get_real_ip(request)

    # 如果有客户端哈希：优先信任它并立即可下载；随后后台验证
    app_obj = current_app._get_current_object()

    if client_hash_valid:
        # 秒传去重：若已存在相同哈希，复用物理文件并删除刚上传文件
        existing_clip = Clip.query.filter_by(file_hash=client_hash).first()

        if existing_clip and existing_clip.file_path and os.path.exists(existing_clip.file_path):
            # 删除刚上传的重复文件
            if os.path.exists(file_path):
                try:
                    os.remove(file_path)
                except OSError:
                    pass
            new_clip = Clip(
                code=code,
                content_type='file',
                filename=original_filename,
                file_path=existing_clip.file_path,
                file_hash=client_hash,
                file_size=existing_clip.file_size,
                mime_type=existing_clip.mime_type or mime_type,
                client_ip=client_ip,
                access_count=count,
                max_count=count,
                expire_seconds=expire
            )
            try:
                db.session.add(new_clip)
                db.session.commit()
            except Exception:
                db.session.rollback()
                return jsonify({"error": "Database error"}), 500

            # 后台验证（即使复用也需验证客户端哈希真实性）
            def _verify_client_hash(app, clip_id, trusted_hash, path_for_verify):
                try:
                    with app.app_context():
                        # 若复用路径不存在，则无需验证
                        if not path_for_verify or not os.path.exists(path_for_verify):
                            return
                        h = hashlib.sha256()
                        with open(path_for_verify, 'rb') as rf:
                            for chunk in iter(lambda: rf.read(1024 * 1024), b''):
                                h.update(chunk)
                        server_hash = h.hexdigest()
                        if server_hash != trusted_hash:
                            # 删除记录（复用物理文件不删除）
                            bad = Clip.query.get(clip_id)
                            if bad:
                                db.session.delete(bad)
                                db.session.commit()
                except Exception:
                    db.session.rollback()
                    pass

            t = threading.Thread(target=_verify_client_hash, args=(app_obj, new_clip.id, client_hash, existing_clip.file_path), daemon=True)
            t.start()

            return jsonify({"code": code, "instant_upload": True})

        # 无复用：写入记录，信任客户端哈希并允许下载
        clip = Clip(
            code=code,
            content_type='file',
            filename=original_filename,
            file_path=file_path,
            file_hash=client_hash,
            file_size=file_size,
            mime_type=mime_type,
            client_ip=client_ip,
            access_count=count,
            max_count=count,
            expire_seconds=expire
        )

        try:
            db.session.add(clip)
            db.session.commit()
        except Exception:
            db.session.rollback()
            if os.path.exists(file_path):
                os.remove(file_path)
            return jsonify({"error": "Database error"}), 500

        # 后台验证：若与客户端哈希不一致，删除文件与记录
        def _verify_and_cleanup(app, clip_id, trusted_hash, path_current):
            try:
                with app.app_context():
                    h = hashlib.sha256()
                    with open(path_current, 'rb') as rf:
                        for chunk in iter(lambda: rf.read(1024 * 1024), b''):
                            h.update(chunk)
                    server_hash = h.hexdigest()
                    if server_hash != trusted_hash:
                        # 删除磁盘文件与记录
                        if os.path.exists(path_current):
                            try:
                                os.remove(path_current)
                            except OSError:
                                pass
                        bad = Clip.query.get(clip_id)
                        if bad:
                            db.session.delete(bad)
                            db.session.commit()
            except Exception:
                db.session.rollback()
                pass

        t = threading.Thread(target=_verify_and_cleanup, args=(app_obj, clip.id, client_hash, file_path), daemon=True)
        t.start()

        return jsonify({"code": code, "instant_upload": False})

    # 没有客户端哈希：退化为原策略（后台计算，哈希完成后才开放下载）
    clip = Clip(
        code=code,
        content_type='file',
        filename=original_filename,
        file_path=file_path,
        file_hash=None,
        file_size=file_size,
        mime_type=mime_type,
        client_ip=client_ip,
        access_count=count,
        max_count=count,
        expire_seconds=expire
    )

    try:
        db.session.add(clip)
        db.session.commit()
    except Exception:
        db.session.rollback()
        if os.path.exists(file_path):
            os.remove(file_path)
        return jsonify({"error": "Database error"}), 500

    def _compute_and_finalize(app, clip_id, path):
        try:
            with app.app_context():
                h = hashlib.sha256()
                size = 0
                with open(path, 'rb') as rf:
                    for data in iter(lambda: rf.read(1024 * 1024), b''):
                        h.update(data)
                        size += len(data)
                file_hash_local = h.hexdigest()
                existing = Clip.query.filter(Clip.file_hash == file_hash_local, Clip.id != clip_id).first()
                clip_row = Clip.query.get(clip_id)
                if not clip_row:
                    return
                clip_row.file_hash = file_hash_local
                if existing and existing.file_path and os.path.exists(existing.file_path):
                    if os.path.exists(path):
                        try:
                            os.remove(path)
                        except OSError:
                            pass
                    clip_row.file_path = existing.file_path
                    clip_row.file_size = existing.file_size or size
                    clip_row.mime_type = existing.mime_type or clip_row.mime_type
                else:
                    clip_row.file_size = size
                db.session.commit()
        except Exception:
            db.session.rollback()
            pass

    t = threading.Thread(target=_compute_and_finalize, args=(app_obj, clip.id, file_path), daemon=True)
    t.start()

    return jsonify({"code": code})


def timetask_cleanup_files():
    """API端点用于清理过时文件和孤立文件"""
    try:
        # 清理过期的剪贴板记录
        expired_count = cleanup_expired_clips()

        # 清理孤立文件（存在于目录但不在数据库中的文件）
        orphaned_count = cleanup_orphaned_files()

        # 清理空目录
        empty_dir_count = cleanup_empty_directories()

        return {
            "expired": expired_count,
            "orphaned_files": orphaned_count,
            "empty_dirs": empty_dir_count,
            "message": f"Cleaned {expired_count} expired, {orphaned_count} orphaned files, {empty_dir_count} empty dirs"
        }
    except Exception as e:
        return {"error": f"Cleanup failed: {str(e)}"}


def cleanup_orphaned_files():
    """清理孤立文件：删除存在于目录但不在数据库中的文件"""

    orphaned_count = 0
    data_dir = "data"

    if not os.path.exists(data_dir):
        return orphaned_count

    try:
        # 获取数据库中所有有效的文件路径
        valid_file_paths = set()
        active_clips = Clip.query.filter(Clip.content_type == 'file').all()

        for clip in active_clips:
            if clip.file_path and not clip.is_expired():
                valid_file_paths.add(os.path.normpath(clip.file_path))

        # 扫描data目录下的所有文件
        pattern = os.path.join(data_dir, "*", "*")
        disk_files = glob.glob(pattern)

        for file_path in disk_files:
            # 跳过目录
            if os.path.isdir(file_path):
                continue

            normalized_path = os.path.normpath(file_path)

            # 如果文件不在数据库的有效文件列表中，则删除
            if normalized_path not in valid_file_paths:
                try:
                    os.remove(file_path)
                    orphaned_count += 1
                    print(f"删除孤立文件: {file_path}")
                except OSError as e:
                    print(f"删除孤立文件失败 {file_path}: {e}")

        return orphaned_count

    except Exception as e:
        print(f"清理孤立文件时出错: {e}")
        raise e


def cleanup_empty_directories(base_dir: str = "data") -> int:
    """清理空目录：仅删除 data/ 下的空文件夹（自底向上），返回删除数量。

    约束：
    - 只处理 `base_dir` 目录内的子目录，不删除 `base_dir` 本身。
    - 自底向上遍历，确保先删叶子节点避免残留。
    - 遇到异常（权限/并发删除）时记录日志但不中断整体流程。
    """
    removed_count = 0

    if not os.path.exists(base_dir):
        return removed_count

    try:
        for root, dirs, files in os.walk(base_dir, topdown=False):
            # 跳过根目录本身
            if os.path.normpath(root) == os.path.normpath(base_dir):
                continue

            try:
                # 目录为空（无文件无子目录）则删除
                if not os.listdir(root):
                    os.rmdir(root)
                    removed_count += 1
                    print(f"删除空目录: {root}")
            except OSError as e:
                # 常见于并发或权限问题，忽略并继续
                print(f"删除空目录失败 {root}: {e}")

        return removed_count
    except Exception as e:
        print(f"清理空目录时出错: {e}")
        raise e
