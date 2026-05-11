from fastapi import APIRouter, Request, Depends, Form, UploadFile, File, BackgroundTasks, Response
from fastapi.responses import JSONResponse, RedirectResponse, FileResponse
from sqlalchemy.orm import Session
from app.database import Clip, get_db, cleanup_expired_clips
import random
import validators
import hashlib
import os
import mimetypes
import glob
from datetime import datetime
from app.utils import get_real_ip
from app.config import settings

router = APIRouter()

def cleanup_orphaned_files(db: Session):
    orphaned_count = 0
    data_dir = "data"
    if not os.path.exists(data_dir):
        return orphaned_count

    try:
        valid_file_paths = set()
        active_clips = db.query(Clip).filter(Clip.content_type == 'file').all()

        for clip in active_clips:
            if clip.file_path and not clip.is_expired():
                valid_file_paths.add(os.path.normpath(clip.file_path))

        pattern = os.path.join(data_dir, "*", "*")
        disk_files = glob.glob(pattern)

        for file_path in disk_files:
            if os.path.isdir(file_path):
                continue
            normalized_path = os.path.normpath(file_path)
            if normalized_path not in valid_file_paths:
                try:
                    os.remove(file_path)
                    orphaned_count += 1
                except OSError as e:
                    pass

        return orphaned_count
    except Exception as e:
        return 0

def cleanup_empty_directories(base_dir: str = "data") -> int:
    removed_count = 0
    if not os.path.exists(base_dir):
        return removed_count

    try:
        for root, dirs, files in os.walk(base_dir, topdown=False):
            if os.path.normpath(root) == os.path.normpath(base_dir):
                continue
            try:
                if not os.listdir(root):
                    os.rmdir(root)
                    removed_count += 1
            except OSError as e:
                pass
        return removed_count
    except Exception as e:
        return 0

@router.post("/create")
def create_clip(
    request: Request,
    count: int = Form(1),
    expire: int = Form(3600),
    link: str = Form("no"),
    content: str = Form(""),
    db: Session = Depends(get_db)
):
    is_link = link.lower() == "yes"
    content = content.strip()

    if not content:
        return JSONResponse({"error": "Missing content"}, status_code=400)

    if is_link and not validators.url(content):
        return JSONResponse({"error": "Content must be a valid URL when link=yes"}, status_code=400)
    
    if is_link and len(content) > settings.MAX_LINK_LENGTH:
        return JSONResponse({"error": "Link is too long"}, status_code=400)

    if not is_link:
        if len(content.encode('utf-8')) > settings.MAX_TEXT_SIZE:
            return JSONResponse({"error": "Text content too large"}, status_code=400)

    content_type = "link" if is_link else "text/plain"
    
    while True:
        code = str(random.randint(10000, 99999))
        if not db.query(Clip).filter_by(code=code).first():
            break

    client_ip = get_real_ip(request)

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
        db.add(clip)
        db.commit()
        return {"code": code}
    except Exception as e:
        db.rollback()
        return JSONResponse({"error": "Database error"}, status_code=500)

@router.get("/{code}")
def get_clip(code: str, db: Session = Depends(get_db)):
    clip = db.query(Clip).filter_by(code=code).first()
    if not clip:
        return JSONResponse({"error": "Not found"}, status_code=404)

    if clip.is_expired():
        if clip.content_type == 'file' and clip.file_path:
            try:
                other_refs = 0
                if clip.file_hash:
                    other_refs = db.query(Clip).filter(
                        Clip.file_hash == clip.file_hash,
                        Clip.id != clip.id
                    ).count()
                if other_refs == 0 and os.path.exists(clip.file_path):
                    os.remove(clip.file_path)
            except OSError:
                pass

        db.delete(clip)
        db.commit()
        return JSONResponse({"error": "Not found"}, status_code=404)

    if clip.content_type == "link":
        if clip.access_count <= 0:
            db.delete(clip)
            db.commit()
            return JSONResponse({"error": "Not found"}, status_code=404)

        clip.access_count -= 1
        if clip.access_count <= 0:
            db.delete(clip)
        db.commit()

        return RedirectResponse(url=clip.content, status_code=302)

    if clip.content_type == "file":
        full_file_path = os.path.abspath(clip.file_path)

        if not os.path.exists(full_file_path):
            db.delete(clip)
            db.commit()
            return JSONResponse({"error": "Not found"}, status_code=404)

        mime_type = clip.mime_type or 'application/octet-stream'
        viewable_types = ['text/', 'application/json', 'application/xml', 'application/javascript']
        is_viewable = any(mime_type.startswith(vtype) for vtype in viewable_types)
        
        headers = {}
        if not is_viewable:
            headers["Content-Disposition"] = f'attachment; filename="{clip.filename or "file"}"'
            
        return FileResponse(full_file_path, media_type=mime_type, headers=headers)

    clip.access_count -= 1
    if clip.access_count <= 0:
        db.delete(clip)
    db.commit()

    content_type = clip.content_type
    media_type = f"{content_type}; charset=utf-8" if content_type != "text/plain" else "text/plain; charset=utf-8"
    return Response(content=clip.content, media_type=media_type)

@router.post("/upload")
def upload_file(
    request: Request,
    background_tasks: BackgroundTasks,
    count: int = Form(1000),
    expire: int = Form(604800),
    client_sha256: str = Form(""),
    file: UploadFile = File(...),
    db: Session = Depends(get_db)
):
    if not file or not file.filename:
        return JSONResponse({"error": "No file selected"}, status_code=400)

    current_date = datetime.now().strftime("%Y-%m-%d")
    upload_dir = os.path.join("data", current_date)
    os.makedirs(upload_dir, exist_ok=True)

    while True:
        code = str(random.randint(10000, 99999))
        if not db.query(Clip).filter_by(code=code).first():
            break

    original_filename = file.filename or f"file_{code}"
    client_hash = client_sha256.strip().lower()

    def _is_valid_sha256(h: str) -> bool:
        return len(h) == 64 and all(c in '0123456789abcdef' for c in h)
    client_hash_valid = _is_valid_sha256(client_hash)

    def sanitize_filename(name: str) -> str:
        base = os.path.basename(name)
        illegal = '<>:"/\\|?*'
        cleaned = ''.join('_' if (ch in illegal or ord(ch) < 32) else ch for ch in base)
        return cleaned or f"file_{code}"

    safe_name_for_path = sanitize_filename(original_filename)
    file_path = os.path.join(upload_dir, f"{code}_{safe_name_for_path}")

    file_size = 0
    max_file_size = settings.MAX_UPLOAD_FILE_SIZE
    
    try:
        with open(file_path, 'wb') as f:
            while True:
                chunk = file.file.read(8192)
                if not chunk:
                    break
                f.write(chunk)
                file_size += len(chunk)
                if file_size > max_file_size:
                    if os.path.exists(file_path):
                        os.remove(file_path)
                    return JSONResponse({"error": "File too large"}, status_code=413)
    except Exception as e:
        if os.path.exists(file_path):
            os.remove(file_path)
        return JSONResponse({"error": f"Failed to save file: {str(e)}"}, status_code=500)

    mime_type, _ = mimetypes.guess_type(original_filename)
    if not mime_type:
        mime_type = "application/octet-stream"

    client_ip = get_real_ip(request)

    if client_hash_valid:
        existing_clip = db.query(Clip).filter_by(file_hash=client_hash).first()

        if existing_clip and existing_clip.file_path and os.path.exists(existing_clip.file_path):
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
                db.add(new_clip)
                db.commit()
            except Exception:
                db.rollback()
                return JSONResponse({"error": "Database error"}, status_code=500)

            def _verify_client_hash(clip_id, trusted_hash, path_for_verify):
                from app.database import SessionLocal
                with SessionLocal() as db_session:
                    if not path_for_verify or not os.path.exists(path_for_verify):
                        return
                    h = hashlib.sha256()
                    with open(path_for_verify, 'rb') as rf:
                        for chunk in iter(lambda: rf.read(1024 * 1024), b''):
                            h.update(chunk)
                    server_hash = h.hexdigest()
                    if server_hash != trusted_hash:
                        bad = db_session.query(Clip).get(clip_id)
                        if bad:
                            db_session.delete(bad)
                            db_session.commit()

            background_tasks.add_task(_verify_client_hash, new_clip.id, client_hash, existing_clip.file_path)
            return {"code": code, "instant_upload": True}

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
            db.add(clip)
            db.commit()
        except Exception:
            db.rollback()
            if os.path.exists(file_path):
                os.remove(file_path)
            return JSONResponse({"error": "Database error"}, status_code=500)

        def _verify_and_cleanup(clip_id, trusted_hash, path_current):
            from app.database import SessionLocal
            with SessionLocal() as db_session:
                h = hashlib.sha256()
                with open(path_current, 'rb') as rf:
                    for chunk in iter(lambda: rf.read(1024 * 1024), b''):
                        h.update(chunk)
                server_hash = h.hexdigest()
                if server_hash != trusted_hash:
                    if os.path.exists(path_current):
                        try:
                            os.remove(path_current)
                        except OSError:
                            pass
                    bad = db_session.query(Clip).get(clip_id)
                    if bad:
                        db_session.delete(bad)
                        db_session.commit()

        background_tasks.add_task(_verify_and_cleanup, clip.id, client_hash, file_path)
        return {"code": code, "instant_upload": False}

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
        db.add(clip)
        db.commit()
    except Exception:
        db.rollback()
        if os.path.exists(file_path):
            os.remove(file_path)
        return JSONResponse({"error": "Database error"}, status_code=500)

    def _compute_and_finalize(clip_id, path):
        from app.database import SessionLocal
        with SessionLocal() as db_session:
            try:
                h = hashlib.sha256()
                size = 0
                with open(path, 'rb') as rf:
                    for data in iter(lambda: rf.read(1024 * 1024), b''):
                        h.update(data)
                        size += len(data)
                file_hash_local = h.hexdigest()
                existing = db_session.query(Clip).filter(Clip.file_hash == file_hash_local, Clip.id != clip_id).first()
                clip_row = db_session.query(Clip).get(clip_id)
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
                db_session.commit()
            except Exception:
                db_session.rollback()

    background_tasks.add_task(_compute_and_finalize, clip.id, file_path)
    return {"code": code}

@router.get("/timetask_cleanup_files")
def timetask_cleanup_files(db: Session = Depends(get_db)):
    try:
        expired_count = cleanup_expired_clips(db)
        orphaned_count = cleanup_orphaned_files(db)
        empty_dir_count = cleanup_empty_directories()
        return {
            "expired": expired_count,
            "orphaned_files": orphaned_count,
            "empty_dirs": empty_dir_count,
            "message": f"Cleaned {expired_count} expired, {orphaned_count} orphaned files, {empty_dir_count} empty dirs"
        }
    except Exception as e:
        return JSONResponse({"error": f"Cleanup failed: {str(e)}"}, status_code=500)
