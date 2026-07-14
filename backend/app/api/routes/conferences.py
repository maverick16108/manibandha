import time
import uuid

import jwt
from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy import case
from sqlalchemy.orm import Session, joinedload

from app.api.deps import get_current_user, require_cap
from app.core.capabilities import has_cap
from app.core.config import settings
from app.core.database import get_db
from app.models import Conference, User
from app.schemas.conference import ConferenceCreate, ConferenceOut, ConferenceUpdate, JoinOut

router = APIRouter(prefix="/conferences", tags=["conferences"])


def _mint_token(identity: str, name: str, room: str, can_publish: bool, ttl: int = 6 * 3600) -> str:
    """LiveKit access token (JWT, HS256)."""
    now = int(time.time())
    claims = {
        "iss": settings.LIVEKIT_API_KEY,
        "sub": identity,
        "name": name,
        "nbf": now - 5,
        "exp": now + ttl,
        "video": {
            "room": room,
            "roomJoin": True,
            "canPublish": can_publish,
            "canSubscribe": True,
            "canPublishData": True,
        },
    }
    return jwt.encode(claims, settings.LIVEKIT_API_SECRET, algorithm="HS256")


def _out(c: Conference, user: User, is_host_cap: bool) -> ConferenceOut:
    return ConferenceOut(
        id=c.id, title=c.title, description=c.description, mode=c.mode, status=c.status,
        host_id=c.host_id, host_name=c.host.full_name if c.host else None,
        can_host=(c.host_id == user.id or is_host_cap),
        scheduled_at=c.scheduled_at, started_at=c.started_at, ended_at=c.ended_at, created_at=c.created_at,
    )


@router.get("", response_model=list[ConferenceOut])
def list_conferences(db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    is_host_cap = has_cap(db, user, "conference.host")
    order = case((Conference.status == "live", 0), (Conference.status == "scheduled", 1), else_=2)
    rows = (
        db.query(Conference).options(joinedload(Conference.host))
        .order_by(order, Conference.scheduled_at.is_(None), Conference.scheduled_at.asc(), Conference.created_at.desc())
        .all()
    )
    return [_out(c, user, is_host_cap) for c in rows]


@router.post("", response_model=ConferenceOut, status_code=status.HTTP_201_CREATED)
def create_conference(payload: ConferenceCreate, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.host"))):
    title = (payload.title or "").strip()
    if not title:
        raise HTTPException(status_code=400, detail="Нужно название конференции")
    mode = payload.mode if payload.mode in ("interactive", "broadcast") else "interactive"
    c = Conference(
        title=title[:255], description=(payload.description or "").strip() or None,
        mode=mode, room=f"conf_{uuid.uuid4().hex[:20]}", host_id=user.id,
        scheduled_at=payload.scheduled_at, status="scheduled",
    )
    db.add(c)
    db.commit()
    db.refresh(c)
    return _out(c, user, True)


def _editable(db: Session, user: User, c: Conference):
    if c.host_id != user.id and not has_cap(db, user, "conference.host"):
        raise HTTPException(status_code=403, detail="Управлять может ведущий или организатор")


@router.patch("/{conf_id}", response_model=ConferenceOut)
def update_conference(conf_id: int, payload: ConferenceUpdate, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    if payload.title is not None:
        t = payload.title.strip()
        if t:
            c.title = t[:255]
    if payload.description is not None:
        c.description = payload.description.strip() or None
    if payload.scheduled_at is not None:
        c.scheduled_at = payload.scheduled_at
    if payload.status in ("scheduled", "live", "ended"):
        c.status = payload.status
        from sqlalchemy import func as _f
        if payload.status == "live" and not c.started_at:
            c.started_at = _f.now()
        if payload.status == "ended":
            c.ended_at = _f.now()
    db.commit()
    db.refresh(c)
    return _out(c, user, has_cap(db, user, "conference.host"))


@router.delete("/{conf_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_conference(conf_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    db.delete(c)
    db.commit()


@router.post("/{conf_id}/join", response_model=JoinOut)
def join_conference(conf_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    if not (settings.LIVEKIT_API_KEY and settings.LIVEKIT_API_SECRET and settings.LIVEKIT_URL):
        raise HTTPException(status_code=503, detail="Видеосервер не настроен")
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    if c.status == "ended":
        raise HTTPException(status_code=409, detail="Конференция завершена")
    is_host = c.host_id == user.id or has_cap(db, user, "conference.host")
    # интерактив — публикуют все; трансляция — только ведущий
    can_publish = is_host or c.mode == "interactive"
    # ведущий заходит — помечаем «идёт»
    if is_host and c.status == "scheduled":
        from sqlalchemy import func as _f
        c.status = "live"
        c.started_at = _f.now()
        db.commit()
    identity = f"u{user.id}"
    token = _mint_token(identity, user.full_name or "Гость", c.room, can_publish)
    return JoinOut(url=settings.LIVEKIT_URL, token=token, room=c.room, mode=c.mode,
                   can_publish=can_publish, identity=identity)
