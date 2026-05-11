import uvicorn
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse, JSONResponse, Response
from app.routes import api_router
from app.database import init_database
import os

app = FastAPI(title="ClipBox")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Initialize database
init_database()

# Include API routers
app.include_router(api_router, prefix="")

# Serve frontend
project_root = os.path.dirname(__file__)
www_root = os.path.join(project_root, "www")

if os.path.exists(www_root):
    app.mount(
        "/assets",
        StaticFiles(directory=os.path.join(www_root, "assets")),
        name="assets",
    )

    @app.get("/{full_path:path}")
    def serve_frontend(full_path: str):
        # Allow requests to /api to pass through to the 404 handler if they don't match
        if full_path.startswith("api/"):
            return JSONResponse({"detail": "Not Found"}, status_code=404)

        # If it's a file request (e.g., .js, .css, .ico)
        path = os.path.join(www_root, full_path)
        if os.path.isfile(path):
            return FileResponse(path)

        # Try finding an index.html if it's a directory
        index_path = os.path.join(path, "index.html")
        if os.path.isfile(index_path):
            return FileResponse(index_path)

        # Fallback to SPA root index.html
        return FileResponse(os.path.join(www_root, "index.html"))
else:

    @app.get("/{full_path:path}")
    def api_fallback(full_path: str):
        return Response("Clipbox-API", media_type="text/plain")


if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=5328, reload=False)
