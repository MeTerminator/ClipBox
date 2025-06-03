import os


def get_redis_url():
    url = os.getenv("REDIS_URL")
    if url:
        url = url.strip('"')
    return url


class Config:
    REDIS_URL = get_redis_url()
