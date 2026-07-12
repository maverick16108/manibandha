from datetime import datetime

from pydantic import BaseModel, ConfigDict

from app.core.enums import ThreadKind


class MessageCreate(BaseModel):
    body: str


class MessageOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    author_id: int | None
    author_name: str | None = None
    body: str
    created_at: datetime
    edit_count: int = 0
    likes: int = 0
    liked: bool = False


class ThreadCreate(BaseModel):
    kind: ThreadKind
    body: str
    disciple_id: int | None = None  # только для гуру; ученик — по своей анкете
    subject: str | None = None
    period: str | None = None  # 'YYYY-MM' для отчётов


class ThreadListItem(BaseModel):
    id: int
    kind: ThreadKind
    disciple_id: int
    disciple_name: str
    subject: str | None
    period: str | None
    updated_at: datetime
    messages_count: int
    last_preview: str | None
    unread: bool = False


class ThreadOut(BaseModel):
    id: int
    kind: ThreadKind
    disciple_id: int
    disciple_name: str
    subject: str | None
    period: str | None
    created_at: datetime
    updated_at: datetime
    messages: list[MessageOut]
