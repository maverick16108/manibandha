from datetime import datetime

from pydantic import BaseModel


class ConferenceCreate(BaseModel):
    title: str
    description: str | None = None
    mode: str = "interactive"  # interactive | broadcast
    scheduled_at: datetime | None = None
    mic_allowed: bool = True
    cam_allowed: bool = True
    screen_allowed: bool = True
    guests_allowed: bool = False
    auto_record: bool = False
    host_id: int | None = None


class ConferenceUpdate(BaseModel):
    title: str | None = None
    description: str | None = None
    scheduled_at: datetime | None = None
    status: str | None = None  # scheduled | live | ended
    mode: str | None = None
    mic_allowed: bool | None = None
    cam_allowed: bool | None = None
    screen_allowed: bool | None = None
    guests_allowed: bool | None = None
    auto_record: bool | None = None
    host_id: int | None = None


class ConferenceParticipant(BaseModel):
    name: str
    avatar_url: str | None = None


class ConferenceOut(BaseModel):
    id: int
    title: str
    description: str | None = None
    mode: str
    status: str
    room: str | None = None
    code: str | None = None
    mic_allowed: bool = True
    cam_allowed: bool = True
    screen_allowed: bool = True
    guests_allowed: bool = False
    auto_record: bool = False
    host_id: int | None = None
    host_name: str | None = None
    can_host: bool = False
    scheduled_at: datetime | None = None
    started_at: datetime | None = None
    ended_at: datetime | None = None
    created_at: datetime
    # для карточки «в эфире»: сколько сейчас участников и первые из них
    participant_count: int = 0
    participants: list[ConferenceParticipant] = []


class JoinOut(BaseModel):
    url: str
    token: str
    room: str
    code: str | None = None
    mode: str
    title: str | None = None
    can_publish: bool
    is_host: bool = False
    identity: str
    # текущие разрешения «всем» — чтобы переключатели ведущего стартовали в правильном положении
    mic_allowed: bool = True
    cam_allowed: bool = True
    screen_allowed: bool = True
