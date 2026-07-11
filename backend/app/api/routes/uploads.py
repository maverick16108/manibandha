import os
import uuid

from fastapi import APIRouter, Depends, File, HTTPException, UploadFile

from app.api.deps import get_current_user
from app.core.config import settings
from app.models import User

router = APIRouter(prefix="/uploads", tags=["uploads"])

ALLOWED = {"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif"}
MAX_BYTES = 8 * 1024 * 1024  # 8 MB per file


@router.post("")
async def upload(files: list[UploadFile] = File(...), _: User = Depends(get_current_user)):
    os.makedirs(settings.UPLOAD_DIR, exist_ok=True)
    urls = []
    for f in files:
        ext = ALLOWED.get(f.content_type)
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
