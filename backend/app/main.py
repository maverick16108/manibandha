from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.routes import auth, cities, countries, disciples, pipeline, regions, reports, temples, users
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
          disciples.router, pipeline.router, reports.router):
    app.include_router(r, prefix=settings.API_PREFIX)


@app.get(f"{settings.API_PREFIX}/health", tags=["health"])
def health():
    return {"status": "ok", "app": settings.APP_NAME}
