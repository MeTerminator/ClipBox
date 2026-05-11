import os
from pydantic_settings import BaseSettings

# https://github.com/MeTerminator/ClipBox

class Settings(BaseSettings):
    # MySQL数据库配置
    DATABASE_URL: str = 'mysql+pymysql://root:root@127.0.0.1:3306/clipbox?charset=utf8mb4'
    
    # 真实IP头部
    REAL_IP_HEADER: str = 'X-Real-IP'

    # 限制配置
    MAX_TEXT_SIZE: int = 5 * 1024 * 1024
    MAX_LINK_LENGTH: int = 2048
    MAX_UPLOAD_FILE_SIZE: int = 500 * 1024 * 1024

    class Config:
        env_file = ".env"

settings = Settings()
