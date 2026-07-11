from pydantic import BaseModel, ConfigDict


class TempleBase(BaseModel):
    name: str
    city: str | None = None
    country: str | None = None
    president_name: str | None = None
    notes: str | None = None


class TempleCreate(TempleBase):
    pass


class TempleUpdate(BaseModel):
    name: str | None = None
    city: str | None = None
    country: str | None = None
    president_name: str | None = None
    notes: str | None = None


class TempleOut(TempleBase):
    model_config = ConfigDict(from_attributes=True)
    id: int
