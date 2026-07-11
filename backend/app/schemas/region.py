from pydantic import BaseModel, ConfigDict


class RegionBase(BaseModel):
    name: str


class RegionCreate(RegionBase):
    pass


class RegionUpdate(BaseModel):
    name: str | None = None


class RegionOut(RegionBase):
    model_config = ConfigDict(from_attributes=True)
    id: int
