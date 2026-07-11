import os

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

from app.api.routes import (
    auth, cities, countries, disciples, drafts, events, mentors, permissions, pipeline, regions, reports, temples,
    threads, uploads, users,
)
from app.core.config import settings

app = FastAPI(title=settings.APP_NAME, openapi_url=f"{settings.API_PREFIX}/openapi.json",
              docs_url=f"{settings.API_PREFIX}/docs")

app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

for r in (auth.router, users.router, temples.router, cities.router, countries.router, regions.router,
          disciples.router, pipeline.router, reports.router, uploads.router, mentors.router, threads.router,
          events.router, permissions.router, drafts.router):
    app.include_router(r, prefix=settings.API_PREFIX)

# WebSocket for interactive chat (typing + instant delivery)
from app.api import ws as ws_module  # noqa: E402

app.include_router(ws_module.router, prefix=settings.API_PREFIX)

# Serve uploaded images (dev; in prod nginx also serves /uploads directly).
os.makedirs(settings.UPLOAD_DIR, exist_ok=True)
app.mount("/uploads", StaticFiles(directory=settings.UPLOAD_DIR), name="uploads")


@app.get(f"{settings.API_PREFIX}/health", tags=["health"])
def health():
    return {"status": "ok", "app": settings.APP_NAME}
