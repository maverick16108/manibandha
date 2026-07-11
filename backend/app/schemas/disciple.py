from datetime import date, datetime

from pydantic import BaseModel, ConfigDict

from app.core.enums import InitiationStatus, MaritalStatus
from app.schemas.checklist import ChecklistItemOut
from app.schemas.temple import TempleOut


class MentorBrief(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    name: str


class DiscipleBase(BaseModel):
    spiritual_name: str | None = None
    material_name: str
    photo_url: str | None = None

    phone: str | None = None
    email: str | None = None
    messenger: str | None = None

    country: str | None = None
    region: str | None = None
    city: str | None = None
    temple_id: int | None = None

    marital_status: MaritalStatus | None = None
    date_of_birth: date | None = None

    initiation_status: InitiationStatus = InitiationStatus.aspirant
    pranama_date: date | None = None
    harinama_date: date | None = None
    harinama_name: str | None = None
    brahman_date: date | None = None

    seva: str | None = None
    current_activity: str | None = None

    mentor_id: int | None = None
    is_mentor: bool = False
    recommended_by: str | None = None
    application_date: date | None = None
    ready_for_pranama: bool = False
    ready_for_initiation: bool = False

    notes: str | None = None


class DiscipleCreate(DiscipleBase):
    pass


class DiscipleUpdate(BaseModel):
    spiritual_name: str | None = None
    material_name: str | None = None
    photo_url: str | None = None
    phone: str | None = None
    email: str | None = None
    messenger: str | None = None
    country: str | None = None
    region: str | None = None
    city: str | None = None
    temple_id: int | None = None
    marital_status: MaritalStatus | None = None
    date_of_birth: date | None = None
    initiation_status: InitiationStatus | None = None
    pranama_date: date | None = None
    harinama_date: date | None = None
    harinama_name: str | None = None
    brahman_date: date | None = None
    seva: str | None = None
    current_activity: str | None = None
    mentor_id: int | None = None
    is_mentor: bool | None = None
    recommended_by: str | None = None
    application_date: date | None = None
    ready_for_pranama: bool | None = None
    ready_for_initiation: bool | None = None
    notes: str | None = None


class DiscipleListItem(BaseModel):
    """Compact row for list/table views."""
    model_config = ConfigDict(from_attributes=True)
    id: int
    spiritual_name: str | None
    material_name: str
    photo_url: str | None
    country: str | None
    region: str | None
    city: str | None
    initiation_status: InitiationStatus
    is_mentor: bool = False
    is_approved: bool = True
    pranama_date: date | None = None
    harinama_date: date | None = None
    brahman_date: date | None = None
    temple: TempleOut | None = None
    mentor: MentorBrief | None = None


class DiscipleOut(DiscipleBase):
    model_config = ConfigDict(from_attributes=True)
    id: int
    is_approved: bool = True
    created_at: datetime
    updated_at: datetime
    temple: TempleOut | None = None
    mentor: MentorBrief | None = None
    checklist: list[ChecklistItemOut] = []


class DiscipleListResponse(BaseModel):
    total: int
    items: list[DiscipleListItem]
