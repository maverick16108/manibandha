from datetime import datetime

from pydantic import BaseModel, ConfigDict


class NoteCreate(BaseModel):
    text: str


class NoteOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    author_id: int | None
    author_name: str | None = None
    text: str
    created_at: datetime


class FileOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    name: str
    url: str
    size: int | None = None
    content_type: str | None = None
    uploaded_by_name: str | None = None
    created_at: datetime
