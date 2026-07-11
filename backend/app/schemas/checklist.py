from pydantic import BaseModel, ConfigDict

from app.core.enums import InitiationStatus


class ChecklistItemBase(BaseModel):
    title: str
    is_done: bool = False
    note: str | None = None
    target: InitiationStatus = InitiationStatus.harinama


class ChecklistItemCreate(ChecklistItemBase):
    pass


class ChecklistItemUpdate(BaseModel):
    title: str | None = None
    is_done: bool | None = None
    note: str | None = None
    target: InitiationStatus | None = None


class ChecklistItemOut(ChecklistItemBase):
    model_config = ConfigDict(from_attributes=True)
    id: int
    disciple_id: int
