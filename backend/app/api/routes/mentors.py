"""Наставники как справочник. Технически это пользователи с ролью curator
(чтобы работало ограничение «наставник видит своих учеников»), но управляются
простым списком: имя добавляется/переименовывается/удаляется.
"""
import uuid

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.api.deps import staff_user
from app.core.database import get_db
from app.core.enums import Role
from app.core.security import hash_password
from app.models import User
from pydantic import BaseModel


class MentorIn(BaseModel):
    name: str


class MentorOut(BaseModel):
    id: int
    name: str


router = APIRouter(prefix="/mentors", tags=["mentors"])


@router.get("", response_model=list[MentorOut])
def list_mentors(db: Session = Depends(get_db), _: User = Depends(staff_user)):
    rows = db.query(User).filter(User.role == Role.curator).order_by(User.full_name).all()
    return [MentorOut(id=u.id, name=u.full_name) for u in rows]


@router.post("", response_model=MentorOut, status_code=status.HTTP_201_CREATED)
def create_mentor(payload: MentorIn, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    name = payload.name.strip()
    if not name:
        raise HTTPException(status_code=400, detail="Имя наставника обязательно")
    # inert account (random password) — guru can set real credentials later via «Пользователи»
    user = User(
        full_name=name,
        email=f"mentor-{uuid.uuid4().hex[:10]}@manibandha.local",
        role=Role.curator,
        is_active=True,
        hashed_password=hash_password(uuid.uuid4().hex),
    )
    db.add(user)
    db.commit()
    db.refresh(user)
    return MentorOut(id=user.id, name=user.full_name)


@router.patch("/{mentor_id}", response_model=MentorOut)
def rename_mentor(mentor_id: int, payload: MentorIn, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    user = db.get(User, mentor_id)
    if not user or user.role != Role.curator:
        raise HTTPException(status_code=404, detail="Наставник не найден")
    if payload.name.strip():
        user.full_name = payload.name.strip()
    db.commit()
    return MentorOut(id=user.id, name=user.full_name)


@router.delete("/{mentor_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_mentor(mentor_id: int, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    user = db.get(User, mentor_id)
    if not user or user.role != Role.curator:
        raise HTTPException(status_code=404, detail="Наставник не найден")
    db.delete(user)
    db.commit()
