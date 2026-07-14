from datetime import datetime

from pydantic import BaseModel, ConfigDict


class SectionCreate(BaseModel):
    title: str
    description: str | None = None
    color: str | None = None
    cover_url: str | None = None


class SectionUpdate(BaseModel):
    title: str | None = None
    description: str | None = None
    color: str | None = None
    cover_url: str | None = None


class SectionOut(BaseModel):
    id: int
    title: str
    description: str | None = None
    color: str = "#c8742a"
    cover_url: str | None = None
    author_id: int | None = None
    author_name: str | None = None
    topics_count: int = 0
    can_edit: bool = False
    created_at: datetime


class TopicCreate(BaseModel):
    section_id: int
    title: str
    body: str
    cover_url: str | None = None


class PostCreate(BaseModel):
    body: str


class PostOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    author_id: int | None
    author_name: str | None = None
    author_avatar: str | None = None
    body: str
    created_at: datetime
    edit_count: int = 0


class Participant(BaseModel):
    name: str | None = None
    avatar: str | None = None


class TopicListItem(BaseModel):
    id: int
    title: str
    cover_url: str | None = None
    section_id: int | None = None
    section_title: str | None = None
    section_color: str = "#c8742a"
    author_name: str | None = None
    pinned: bool = False
    replies: int = 0
    views: int = 0
    posts_count: int = 0
    participants: list[Participant] = []
    unread: bool = False
    last_activity: datetime
    created_at: datetime


class TopicOut(BaseModel):
    id: int
    title: str
    cover_url: str | None = None
    section_id: int | None = None
    section_title: str | None = None
    section_color: str = "#c8742a"
    author_name: str | None = None
    pinned: bool = False
    created_at: datetime
    posts: list[PostOut]
