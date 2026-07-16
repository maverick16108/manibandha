import os
import uuid

from fastapi import APIRouter, Depends, File, HTTPException, UploadFile

from app.api.deps import get_current_user
from app.core.config import settings
from app.models import User

router = APIRouter(prefix="/uploads", tags=["uploads"])

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
    for f in files:
        ctype = (f.content_type or "").split(";")[0].strip()  # отбросить ;codecs=opus и т.п.
        ext = ALLOWED.get(ctype)
        if not ext:
            raise HTTPException(status_code=400, detail=f"Неподдерживаемый тип файла: {f.content_type}")
        data = await f.read()
        if len(data) > MAX_BYTES:
            raise HTTPException(status_code=400, detail="Файл больше 8 МБ")
        name = f"{uuid.uuid4().hex}{ext}"
        with open(os.path.join(settings.UPLOAD_DIR, name), "wb") as out:
            out.write(data)
        urls.append(f"/uploads/{name}")
    return {"urls": urls}
