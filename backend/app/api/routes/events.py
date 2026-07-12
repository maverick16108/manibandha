from datetime import date

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, staff_user
from app.core.database import get_db
from app.models import Event, User
from app.schemas.event import EventCreate, EventOut, EventUpdate

router = APIRouter(prefix="/events", tags=["events"])


@router.get("", response_model=list[EventOut])
def list_events(db: Session = Depends(get_db), _: User = Depends(get_current_user)):
    return db.query(Event).order_by(Event.starts_on.desc()).all()


def _brief(e: Event) -> dict:
    return {
        "id": e.id,
        "title": e.title,
        "location": e.location,
        "description": e.description,  # для развёрнутой ленты (грид/карта игнорируют)
        "starts_on": e.starts_on.isoformat() if e.starts_on else None,
        "ends_on": e.ends_on.isoformat() if e.ends_on else None,
    }


@router.get("/public/upcoming")
def public_upcoming(db: Session = Depends(get_db)):
    """Публичное расписание для лендинга — предстоящие/текущие события (без деталей)."""
    today = date.today()
    rows = (
        db.query(Event)
        .filter((Event.ends_on >= today) | (Event.ends_on.is_(None) & (Event.starts_on >= today)))
        .order_by(Event.starts_on.asc())
        .limit(8)
        .all()
    )
    return [_brief(e) for e in rows]


@router.get("/public")
def public_list(db: Session = Depends(get_db)):
    """Все события для публичного календаря (без описаний)."""
    rows = db.query(Event).order_by(Event.starts_on.desc()).all()
    return [_brief(e) for e in rows]


@router.get("/public/{event_id}")
def public_detail(event_id: int, db: Session = Depends(get_db)):
    """Полная карточка события для публичной страницы (с описанием)."""
    ev = db.get(Event, event_id)
    if not ev:
        raise HTTPException(status_code=404, detail="Событие не найдено")
    return {**_brief(ev), "description": ev.description}


@router.get("/{event_id}", response_model=EventOut)
def get_event(event_id: int, db: Session = Depends(get_db), _: User = Depends(get_current_user)):
    ev = db.get(Event, event_id)
    if not ev:
        raise HTTPException(status_code=404, detail="Событие не найдено")
    return ev


@router.post("", response_model=EventOut, status_code=status.HTTP_201_CREATED)
def create_event(payload: EventCreate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    ev = Event(**payload.model_dump())
    db.add(ev)
    db.commit()
    db.refresh(ev)
    return ev


@router.patch("/{event_id}", response_model=EventOut)
def update_event(event_id: int, payload: EventUpdate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    ev = db.get(Event, event_id)
    if not ev:
        raise HTTPException(status_code=404, detail="Событие не найдено")
    for k, v in payload.model_dump(exclude_unset=True).items():
        setattr(ev, k, v)
    db.commit()
    db.refresh(ev)
    return ev


@router.delete("/{event_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_event(event_id: int, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    ev = db.get(Event, event_id)
    if not ev:
        raise HTTPException(status_code=404, detail="Событие не найдено")
    db.delete(ev)
    db.commit()
