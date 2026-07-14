from datetime import datetime

from pydantic import BaseModel, ConfigDict


class TopicCreate(BaseModel):
    title: str
    body: str


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


class TopicListItem(BaseModel):
    id: int
    title: str
    author_name: str | None = None
    pinned: bool = False
    created_at: datetime
    updated_at: datetime
    posts_count: int = 0
    last_post_preview: str | None = None
    last_post_author: str | None = None
    last_post_at: datetime | None = None


class TopicOut(BaseModel):
    id: int
    title: str
    author_name: str | None = None
    pinned: bool = False
    created_at: datetime
    posts: list[PostOut]
