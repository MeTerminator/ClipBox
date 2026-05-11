from fastapi import APIRouter
from .clip import router as clip_router

api_router = APIRouter()
api_router.include_router(clip_router, prefix="/clip", tags=["clip"])
