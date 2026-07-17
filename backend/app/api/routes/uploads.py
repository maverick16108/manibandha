import io
import os
import uuid

from fastapi import APIRouter, Depends, File, HTTPException, UploadFile
from PIL import Image, ImageOps

from app.api.deps import get_current_user
from app.core.config import settings
from app.models import User

router = APIRouter(prefix="/uploads", tags=["uploads"])

# Изображения, которые пережимаем в webp (кроме gif — сохраняем анимацию как есть)
IMAGE_TYPES = {"image/jpeg", "image/png", "image/webp"}
MAIN_MAX = 1600   # длинная сторона основного изображения
THUMB_MAX = 320   # длинная сторона превью (аватарки, миниатюры)


def _save_image(data: bytes, stem: str) -> tuple[str, str]:
    """Пережать картинку в webp + сделать лёгкое превью. Возвращает (url, thumb_url)."""
    img = Image.open(io.BytesIO(data))
    img = ImageOps.exif_transpose(img)          # учесть поворот с телефона
    if img.mode in ("RGBA", "LA", "P"):
        img = img.convert("RGBA")
    else:
        img = img.convert("RGB")

    main = img.copy()
    main.thumbnail((MAIN_MAX, MAIN_MAX))
    main_name = f"{stem}.webp"
    main.save(os.path.join(settings.UPLOAD_DIR, main_name), "WEBP", quality=85, method=4)

    thumb = img.copy()
    thumb.thumbnail((THUMB_MAX, THUMB_MAX))
    thumb_name = f"{stem}.thumb.webp"
    thumb.save(os.path.join(settings.UPLOAD_DIR, thumb_name), "WEBP", quality=80, method=4)

    return f"/uploads/{main_name}", f"/uploads/{thumb_name}"

ALLOWED = {
    "image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif",
    # голосовые/аудио
    "audio/webm": ".webm", "audio/ogg": ".ogg", "audio/mpeg": ".mp3",
    "audio/mp4": ".m4a", "audio/x-m4a": ".m4a", "audio/wav": ".wav", "audio/x-wav": ".wav",
    # документы/файлы (отправка вложением в чат)
    "application/pdf": ".pdf",
    "application/msword": ".doc",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
    "application/vnd.ms-excel": ".xls",
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
    "application/vnd.ms-powerpoint": ".ppt",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
    "application/zip": ".zip", "application/x-zip-compressed": ".zip",
    "application/x-rar-compressed": ".rar", "application/vnd.rar": ".rar",
    "application/x-7z-compressed": ".7z",
    "text/plain": ".txt", "text/csv": ".csv",
    "video/mp4": ".mp4", "video/quicktime": ".mov",
}
MAX_BYTES = 16 * 1024 * 1024  # 16 MB per file


@router.post("")
async def upload(files: list[UploadFile] = File(...), _: User = Depends(get_current_user)):
    os.makedirs(settings.UPLOAD_DIR, exist_ok=True)
    urls = []
    thumbs = []
    for f in files:
        ctype = (f.content_type or "").split(";")[0].strip()  # отбросить ;codecs=opus и т.п.
        ext = ALLOWED.get(ctype)
        if not ext:
            raise HTTPException(status_code=400, detail=f"Неподдерживаемый тип файла: {f.content_type}")
        data = await f.read()
        if len(data) > MAX_BYTES:
            raise HTTPException(status_code=400, detail="Файл больше 16 МБ")
        stem = uuid.uuid4().hex
        # Картинки (кроме gif) пережимаем в webp + генерим лёгкое превью
        if ctype in IMAGE_TYPES:
            try:
                url, thumb = _save_image(data, stem)
                urls.append(url)
                thumbs.append(thumb)
                continue
            except Exception:
                pass  # не удалось обработать — сохраняем как есть ниже
        name = f"{stem}{ext}"
        with open(os.path.join(settings.UPLOAD_DIR, name), "wb") as out:
            out.write(data)
        urls.append(f"/uploads/{name}")
        thumbs.append(None)
    return {"urls": urls, "thumbs": thumbs}
