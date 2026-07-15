from fastapi import APIRouter, Body, Depends, HTTPException
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, require_cap
from app.core.database import get_db
from app.models import AppSetting, User

router = APIRouter(prefix="/settings", tags=["settings"])

# Значения по умолчанию
DEFAULTS = {
    "forum_edit_window_minutes": 60,  # сколько минут можно править/удалять своё сообщение
    "auth_expire_days": 30,           # через сколько дней без входа теряется авторизация (скользящее окно)
    "recording_enabled": 1,           # разрешена ли запись конференций
    "recording_height": 720,          # качество записи: высота (480/720/1080) — влияет на нагрузку
}


def get_int_setting(db: Session, key: str, default: int) -> int:
    row = db.get(AppSetting, key)
    if not row:
        return default
    try:
        return int(row.value)
    except (TypeError, ValueError):
        return default


def _set(db: Session, key: str, value: str):
    row = db.get(AppSetting, key)
    if row:
        row.value = value
    else:
        db.add(AppSetting(key=key, value=value))


@router.get("")
def read_settings(db: Session = Depends(get_db), _: User = Depends(get_current_user)):
    """Настройки приложения (для отображения на клиенте)."""
    return {
        "forum_edit_window_minutes": get_int_setting(db, "forum_edit_window_minutes", DEFAULTS["forum_edit_window_minutes"]),
        "auth_expire_days": get_int_setting(db, "auth_expire_days", DEFAULTS["auth_expire_days"]),
        "recording_enabled": bool(get_int_setting(db, "recording_enabled", DEFAULTS["recording_enabled"])),
        "recording_height": get_int_setting(db, "recording_height", DEFAULTS["recording_height"]),
    }


def _apply_int(db: Session, payload: dict, key: str, lo: int, hi: int):
    v = payload.get(key)
    if v is None:
        return
    try:
        v = int(v)
    except (TypeError, ValueError):
        raise HTTPException(status_code=400, detail=f"{key} должно быть числом")
    if v < lo or v > hi:
        raise HTTPException(status_code=400, detail=f"Недопустимое значение для {key}")
    _set(db, key, str(v))


@router.put("")
def update_settings(payload: dict = Body(...), db: Session = Depends(get_db), _: User = Depends(require_cap("settings.manage"))):
    _apply_int(db, payload, "forum_edit_window_minutes", 0, 100000)
    _apply_int(db, payload, "auth_expire_days", 1, 3650)
    if "recording_enabled" in payload:
        _set(db, "recording_enabled", "1" if payload.get("recording_enabled") else "0")
    if payload.get("recording_height") in (480, 720, 1080):
        _set(db, "recording_height", str(int(payload["recording_height"])))
    db.commit()
    return read_settings(db, _)
