from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    APP_NAME: str = "Manibandha"
    ENVIRONMENT: str = "development"
    API_PREFIX: str = "/api"

    # Comma-separated string in .env; exposed as a list via `cors_origins`.
    BACKEND_CORS_ORIGINS: str = "http://localhost:5173"

    SECRET_KEY: str = "change-me"
    ALGORITHM: str = "HS256"
    ACCESS_TOKEN_EXPIRE_MINUTES: int = 720

    DATABASE_URL: str = "postgresql+psycopg2://manibandha:manibandha@localhost:5432/manibandha"

    # Directory for uploaded images (served at /uploads). Absolute on prod, relative in dev.
    UPLOAD_DIR: str = "uploads"

    FIRST_GURU_EMAIL: str = "guru@manibandha.local"
    FIRST_GURU_PASSWORD: str = "change-me"
    FIRST_GURU_NAME: str = "Maharaj"

    @property
    def cors_origins(self) -> list[str]:
        return [o.strip() for o in self.BACKEND_CORS_ORIGINS.split(",") if o.strip()]


settings = Settings()
