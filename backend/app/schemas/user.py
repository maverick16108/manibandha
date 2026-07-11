from datetime import datetime

from pydantic import BaseModel, ConfigDict, EmailStr

from app.core.enums import Role


class UserBase(BaseModel):
    email: EmailStr
    full_name: str
    role: Role = Role.secretary
    is_active: bool = True
    disciple_id: int | None = None


class UserCreate(UserBase):
    password: str


class UserUpdate(BaseModel):
    full_name: str | None = None
    role: Role | None = None
    is_active: bool | None = None
    password: str | None = None
    disciple_id: int | None = None


class UserOut(UserBase):
    model_config = ConfigDict(from_attributes=True)
    id: int
    created_at: datetime


class UserBrief(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    full_name: str
    role: Role
