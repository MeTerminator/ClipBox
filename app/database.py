from flask_sqlalchemy import SQLAlchemy
from datetime import datetime, timedelta
import os
import logging
from sqlalchemy import text
from sqlalchemy.exc import OperationalError

db = SQLAlchemy()

# 配置日志
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class Clip(db.Model):
    __tablename__ = 'mb_clips'
    
    id = db.Column(db.Integer, primary_key=True)
    code = db.Column(db.String(10), unique=True, nullable=False, index=True)
    content_type = db.Column(db.Enum('text/plain', 'link', 'file', name='content_type_enum'), nullable=False)
    content = db.Column(db.Text, nullable=True)  # 文本内容或链接URL
    filename = db.Column(db.String(255), nullable=True)  # 文件名
    file_path = db.Column(db.String(500), nullable=True)  # 文件路径
    file_hash = db.Column(db.String(64), nullable=True, index=True)  # 文件SHA256哈希
    file_size = db.Column(db.BigInteger, nullable=True)  # 文件大小
    mime_type = db.Column(db.String(100), nullable=True)  # MIME类型
    client_ip = db.Column(db.String(45), nullable=True)  # 客户端IP（IPv4/IPv6）
    access_count = db.Column(db.Integer, nullable=False, default=1000)  # 剩余访问次数
    max_count = db.Column(db.Integer, nullable=False, default=1000)  # 最大访问次数
    expire_seconds = db.Column(db.Integer, nullable=False, default=604800)  # 过期时间（秒）
    created_at = db.Column(db.DateTime, default=datetime.utcnow, index=True)
    updated_at = db.Column(db.DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    
    # 已合并到单表结构：不再维护 file_hashes 关系表
    
    def is_expired(self):
        """检查是否过期"""
        if not self.created_at:
            return True
        expire_time = self.created_at + timedelta(seconds=self.expire_seconds)
        return datetime.utcnow() > expire_time
    
    def decrease_access_count(self):
        """减少访问次数"""
        if self.access_count > 0:
            self.access_count -= 1
            return True
        return False
    
    def is_accessible(self):
        """检查是否可访问（未过期且有剩余访问次数）"""
        return not self.is_expired() and self.access_count > 0
    
    def to_dict(self):
        """转换为字典格式"""
        return {
            'id': self.id,
            'code': self.code,
            'content_type': self.content_type,
            'content': self.content,
            'filename': self.filename,
            'file_path': self.file_path,
            'file_hash': self.file_hash,
            'file_size': self.file_size,
            'mime_type': self.mime_type,
            'client_ip': self.client_ip,
            'access_count': self.access_count,
            'max_count': self.max_count,
            'expire_seconds': self.expire_seconds,
            'created_at': self.created_at.isoformat() if self.created_at else None,
            'updated_at': self.updated_at.isoformat() if self.updated_at else None
        }

# 已移除 FileHash 模型：file_hash 直接作为 Clip 字段进行去重查询

def init_database(app):
    """初始化数据库"""
    try:
        with app.app_context():
            db.create_all()
            logger.info("数据库表创建成功")

            # 兼容性迁移：为旧表添加 client_ip 列（如果不存在）
            try:
                # MySQL: 检查列是否存在
                check_sql = text("""
                    SELECT COUNT(*) AS cnt
                    FROM information_schema.COLUMNS
                    WHERE TABLE_SCHEMA = DATABASE()
                      AND TABLE_NAME = 'mb_clips'
                      AND COLUMN_NAME = 'client_ip'
                """)
                result = db.session.execute(check_sql).scalar()
                if not result:
                    db.session.execute(text("ALTER TABLE mb_clips ADD COLUMN client_ip VARCHAR(45) NULL AFTER mime_type"))
                    db.session.commit()
                    logger.info("已为 mb_clips 添加 client_ip 列")
            except OperationalError as e:
                db.session.rollback()
                logger.warning(f"检查/添加 client_ip 列失败: {e}")
    except Exception as e:
        logger.error(f"数据库初始化失败: {e}")
        raise e

def cleanup_expired_clips():
    """清理过期的剪贴板数据"""
    try:
        expired_clips = Clip.query.filter(
            db.func.timestampdiff(db.text('SECOND'), Clip.created_at, db.func.now()) > Clip.expire_seconds
        ).all()
        
        cleaned_count = 0
        for clip in expired_clips:
            # 如果是文件类型，删除磁盘文件
            if clip.content_type == 'file' and clip.file_path:
                try:
                    # 只有当没有其他记录引用同一 file_hash 时，才删除物理文件
                    other_refs = 0
                    if clip.file_hash:
                        other_refs = Clip.query.filter(
                            Clip.file_hash == clip.file_hash,
                            Clip.id != clip.id
                        ).count()
                    if other_refs == 0 and os.path.exists(clip.file_path):
                        os.remove(clip.file_path)
                        logger.info(f"删除文件: {clip.file_path}")
                except OSError as e:
                    logger.warning(f"删除文件失败 {clip.file_path}: {e}")
            
            # 删除数据库记录
            db.session.delete(clip)
            cleaned_count += 1
        
        db.session.commit()
        logger.info(f"清理了 {cleaned_count} 个过期剪贴板记录")
        return cleaned_count
    except Exception as e:
        db.session.rollback()
        logger.error(f"清理过期数据失败: {e}")
        raise e

def get_clip_by_code(code):
    """根据代码获取剪贴板记录"""
    try:
        clip = Clip.query.filter_by(code=code).first()
        if clip and clip.is_accessible():
            return clip
        return None
    except Exception as e:
        logger.error(f"获取剪贴板记录失败 {code}: {e}")
        return None

def create_clip(code, content_type, **kwargs):
    """创建新的剪贴板记录"""
    try:
        clip = Clip(
            code=code,
            content_type=content_type,
            **kwargs
        )
        db.session.add(clip)
        db.session.commit()
        logger.info(f"创建剪贴板记录成功: {code}")
        return clip
    except Exception as e:
        db.session.rollback()
        logger.error(f"创建剪贴板记录失败 {code}: {e}")
        raise e

def update_clip_access(clip):
    """更新剪贴板访问记录"""
    try:
        if clip.decrease_access_count():
            clip.updated_at = datetime.utcnow()
            db.session.commit()
            return True
        return False
    except Exception as e:
        db.session.rollback()
        logger.error(f"更新访问记录失败 {clip.code}: {e}")
        return False

def get_file_by_hash(file_hash):
    """根据文件哈希获取文件记录（用于秒传）"""
    try:
        clip = Clip.query.filter_by(file_hash=file_hash).first()
        if clip and clip.is_accessible():
            return clip
        return None
    except Exception as e:
        logger.error(f"根据哈希获取文件失败 {file_hash}: {e}")
        return None
