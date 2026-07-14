from datetime import datetime

from pydantic import BaseModel


class ConferenceCreate(BaseModel):
    title: str
    description: str | None = None
    mode: str = "interactive"  # interactive | broadcast
    scheduled_at: datetime | None = None


class ConferenceUpdate(BaseModel):
    title: str | None = None
    description: str | None = None
    scheduled_at: datetime | None = None
    status: str | None = None  # scheduled | live | ended


class ConferenceOut(BaseModel):
    id: int
    title: str
    description: str | None = None
    mode: str
    status: str
    host_id: int | None = None
    host_name: str | None = None
    can_host: bool = False
    scheduled_at: datetime | None = None
    started_at: datetime | None = None
    ended_at: datetime | None = None
    created_at: datetime


class JoinOut(BaseModel):
    url: str
    token: str
    room: str
    mode: str
    can_publish: bool
    identity: str
