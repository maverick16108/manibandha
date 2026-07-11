from datetime import date

from pydantic import BaseModel, ConfigDict


class EventBase(BaseModel):
    title: str
    location: str | None = None
    starts_on: date
    ends_on: date | None = None
    description: str | None = None


class EventCreate(EventBase):
    pass


class EventUpdate(BaseModel):
    title: str | None = None
    location: str | None = None
    starts_on: date | None = None
    ends_on: date | None = None
    description: str | None = None


class EventOut(EventBase):
    model_config = ConfigDict(from_attributes=True)
    id: int
