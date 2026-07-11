from pydantic import BaseModel, ConfigDict


class CountryBase(BaseModel):
    name: str


class CountryCreate(CountryBase):
    pass


class CountryUpdate(BaseModel):
    name: str | None = None


class CountryOut(CountryBase):
    model_config = ConfigDict(from_attributes=True)
    id: int
