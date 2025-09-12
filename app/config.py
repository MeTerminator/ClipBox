import os


class Config:
    # MySQL数据库配置
    SQLALCHEMY_DATABASE_URI = os.getenv(
        'DATABASE_URL', 
        'mysql+pymysql://root:root@127.0.0.1:3306/metbox?charset=utf8mb4'
    )
    SQLALCHEMY_TRACK_MODIFICATIONS = False
    SQLALCHEMY_ENGINE_OPTIONS = {
        'pool_recycle': 3600,
        'pool_pre_ping': True
    }

    # 真实IP头部（若存在优先使用该头部）
    REAL_IP_HEADER = os.getenv('REAL_IP_HEADER', 'X-Real-IP')

    # 限制配置（可通过环境变量覆盖）
    # 文本粘贴最大 5MB
    MAX_TEXT_SIZE = int(os.getenv('MAX_TEXT_SIZE', str(5 * 1024 * 1024)))
    # 链接最大长度（默认 2048）
    MAX_LINK_LENGTH = int(os.getenv('MAX_LINK_LENGTH', '2048'))
    # 文件上传最大 500MB
    MAX_UPLOAD_FILE_SIZE = int(os.getenv('MAX_UPLOAD_FILE_SIZE', str(500 * 1024 * 1024)))

    # Flask全局上传大小限制（触发 413 RequestEntityTooLarge）
    MAX_CONTENT_LENGTH = int(os.getenv('MAX_CONTENT_LENGTH', str(500 * 1024 * 1024)))
