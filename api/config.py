import os


class Config:
    REDIS_URL = os.getenv("REDIS_URL")
