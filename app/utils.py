from typing import Optional
from flask import Request, current_app


def get_real_ip(request: Request) -> Optional[str]:
    """
    获取真实客户端IP：
    1) 优先读取配置的头部（默认 X-Real-IP）
    2) 否则回退到 request.remote_addr
    """
    header_name = current_app.config.get('REAL_IP_HEADER', 'X-Real-IP')
    ip = request.headers.get(header_name)

    # 某些代理可能会传递空字符串
    if ip:
        # 去掉可能的多IP（虽然 X-Real-IP 通常只有一个）
        ip = ip.split(',')[0].strip()
        if ip:
            return ip

    return request.remote_addr
