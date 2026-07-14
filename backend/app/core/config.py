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

    # SMSC.ru — отправка SMS-кодов (тот же аккаунт, что в проекте bid)
    SMSC_LOGIN: str = ""
    SMSC_PASSWORD: str = ""
    SMSC_ENABLED: bool = False  # в dev по умолчанию выкл — код пишется в лог
    SMS_CODE_TTL_SECONDS: int = 300

    # LiveKit — видеоконференции (self-hosted)
    LIVEKIT_API_KEY: str = ""
    LIVEKIT_API_SECRET: str = ""
    LIVEKIT_URL: str = ""  # wss://... для клиента
    LIVEKIT_API_URL: str = "http://127.0.0.1:7880"  # HTTP API (RoomService) — внутренний

    @property
    def cors_origins(self) -> list[str]:
        return [o.strip() for o in self.BACKEND_CORS_ORIGINS.split(",") if o.strip()]


settings = Settings()
