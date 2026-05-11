from typing import Optional
from fastapi import Request
from app.config import settings


def get_real_ip(request: Request) -> Optional[str]:
    """
    获取真实客户端IP：
    1) 优先读取配置的头部（默认 X-Real-IP）
    2) 否则回退到 request.client.host
    """
    header_name = settings.REAL_IP_HEADER
    ip = request.headers.get(header_name)

    if ip:
        ip = ip.split(",")[0].strip()
        if ip:
            return ip

    return request.client.host if request.client else None
