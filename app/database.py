from sqlalchemy import create_engine, Column, Integer, String, Enum, Text, BigInteger, DateTime, text
from sqlalchemy.orm import sessionmaker, declarative_base
from datetime import datetime, timedelta
import os
import logging
from app.config import settings

logger = logging.getLogger(__name__)

engine = create_engine(
    settings.DATABASE_URL,
    pool_recycle=3600,
    pool_pre_ping=True
)

SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()

class Clip(Base):
    __tablename__ = 'cb_clips'
    
    id = Column(Integer, primary_key=True)
    code = Column(String(10), unique=True, nullable=False, index=True)
    content_type = Column(Enum('text/plain', 'link', 'file', name='content_type_enum'), nullable=False)
    content = Column(Text, nullable=True)
    filename = Column(String(255), nullable=True)
    file_path = Column(String(500), nullable=True)
    file_hash = Column(String(64), nullable=True, index=True)
    file_size = Column(BigInteger, nullable=True)
    mime_type = Column(String(100), nullable=True)
    client_ip = Column(String(45), nullable=True)
    access_count = Column(Integer, nullable=False, default=1000)
    max_count = Column(Integer, nullable=False, default=1000)
    expire_seconds = Column(Integer, nullable=False, default=604800)
    created_at = Column(DateTime, default=datetime.utcnow, index=True)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    
    def is_expired(self):
        if not self.created_at:
            return True
        expire_time = self.created_at + timedelta(seconds=self.expire_seconds)
        return datetime.utcnow() > expire_time
    
    def decrease_access_count(self):
        if self.access_count > 0:
            self.access_count -= 1
            return True
        return False
    
    def is_accessible(self):
        return not self.is_expired() and self.access_count > 0

def init_database():
    try:
        Base.metadata.create_all(bind=engine)
        logger.info("Database tables created successfully")
        
        with engine.begin() as conn:
            check_sql = text("""
                SELECT COUNT(*) AS cnt
                FROM information_schema.COLUMNS
                WHERE TABLE_SCHEMA = DATABASE()
                  AND TABLE_NAME = 'cb_clips'
                  AND COLUMN_NAME = 'client_ip'
            """)
            result = conn.execute(check_sql).scalar()
            if not result:
                conn.execute(text("ALTER TABLE cb_clips ADD COLUMN client_ip VARCHAR(45) NULL AFTER mime_type"))
                logger.info("Added client_ip column to cb_clips")
    except Exception as e:
        logger.error(f"Database init failed: {e}")

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()

def cleanup_expired_clips(db):
    try:
        # manual cleanup since we are not using complex db specific functions
        expired_clips = db.query(Clip).all()
        expired_clips = [c for c in expired_clips if c.is_expired()]
        
        cleaned_count = 0
        for clip in expired_clips:
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
                        logger.info(f"Deleted file: {clip.file_path}")
                except OSError as e:
                    logger.warning(f"Delete file failed {clip.file_path}: {e}")
            
            db.delete(clip)
            cleaned_count += 1
        
        db.commit()
        return cleaned_count
    except Exception as e:
        db.rollback()
        logger.error(f"Cleanup expired clips failed: {e}")
        return 0
