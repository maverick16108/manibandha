from pydantic import BaseModel, ConfigDict


class CityBase(BaseModel):
    name: str
    country: str | None = None


class CityCreate(CityBase):
    pass


class CityUpdate(BaseModel):
    name: str | None = None
    country: str | None = None


class CityOut(CityBase):
    model_config = ConfigDict(from_attributes=True)
    id: int
