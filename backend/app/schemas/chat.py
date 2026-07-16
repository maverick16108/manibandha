from datetime import datetime

from pydantic import BaseModel, ConfigDict

from app.core.enums import ChatType


class ChatMemberOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    user_id: int
    full_name: str | None = None
    avatar_url: str | None = None
    role: str = "member"
    last_read_seq: int = 0


class ChatMessageOut(BaseModel):
    id: int
    chat_id: int
    seq: int
    client_uuid: str | None = None
    author_id: int | None = None
    author_name: str | None = None
    body: str
    reply_to_id: int | None = None
    reply_preview: str | None = None
    created_at: datetime
    edited_at: datetime | None = None
    edit_count: int = 0
    deleted: bool = False


class ChatOut(BaseModel):
    id: int
    type: ChatType
    title: str | None = None
    photo_url: str | None = None
    created_by: int | None = None
    created_at: datetime
    updated_at: datetime
    members: list[ChatMemberOut] = []
    last_message: ChatMessageOut | None = None
    unread: int = 0  # число непрочитанных для текущего пользователя


class ChatCreateIn(BaseModel):
    type: ChatType
    peer_id: int | None = None            # для личного чата
    title: str | None = None             # для группы
    member_ids: list[int] = []           # для группы (кроме создателя)


class SendMessageIn(BaseModel):
    client_uuid: str
    body: str
    reply_to_id: int | None = None


class EditMessageIn(BaseModel):
    body: str


class ReadIn(BaseModel):
    seq: int  # прочитано до этого seq включительно


class ChatUpdate(BaseModel):
    """Одно событие в потоке синхронизации (getDifference-стиль)."""
    type: str                       # message | edit | delete
    seq: int
    chat_id: int
    message: ChatMessageOut | None = None
    message_id: int | None = None


class UpdatesOut(BaseModel):
    updates: list[ChatUpdate] = []
    pts: int                        # максимальный seq в ответе (новый курсор)
    has_more: bool = False


class ContactOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    full_name: str | None = None
    avatar_url: str | None = None
    role: str | None = None
