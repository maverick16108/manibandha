import json
import time
import urllib.request
import uuid

import jwt
from fastapi import APIRouter, Body, Depends, HTTPException, Request, status
from sqlalchemy import case, func
from sqlalchemy.orm import Session, joinedload

from app.api.deps import require_cap
from app.core.capabilities import has_cap
from app.core.config import settings
from app.core.database import SessionLocal, get_db
from app.models import Conference, User
from app.schemas.conference import ConferenceCreate, ConferenceOut, ConferenceUpdate, JoinOut

router = APIRouter(prefix="/conferences", tags=["conferences"])


def _admin_token(room: str) -> str:
    now = int(time.time())
    return jwt.encode(
        {"iss": settings.LIVEKIT_API_KEY, "nbf": now - 5, "exp": now + 120,
         "video": {"room": room, "roomAdmin": True, "roomJoin": False}},
        settings.LIVEKIT_API_SECRET, algorithm="HS256",
    )


def _room_service(method: str, body: dict, room: str) -> dict:
    """Twirp-вызов LiveKit RoomService (модерация)."""
    url = f"{settings.LIVEKIT_API_URL}/twirp/livekit.RoomService/{method}"
    req = urllib.request.Request(
        url, data=json.dumps(body).encode(),
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {_admin_token(room)}"},
    )
    with urllib.request.urlopen(req, timeout=5) as r:
        return json.loads(r.read().decode() or "{}")


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
    is_host = c.host_id == user.id or has_cap(db, user, "conference.host")
    # интерактив — публикуют все; трансляция — только ведущий
    can_publish = is_host or c.mode == "interactive"
    # переоткрытие завершённой встречи / старт запланированной ведущим
    if c.status == "ended":
        c.status = "live"
        c.ended_at = None
        if not c.started_at:
            c.started_at = func.now()
        db.commit()
    elif is_host and c.status == "scheduled":
        c.status = "live"
        c.started_at = func.now()
        db.commit()
    identity = f"u{user.id}"
    token = _mint_token(identity, user.full_name or "Гость", c.room, can_publish)
    return JoinOut(url=settings.LIVEKIT_URL, token=token, room=c.room, mode=c.mode,
                   can_publish=can_publish, is_host=is_host, identity=identity)


@router.post("/{conf_id}/mute")
def moderate_mute(conf_id: int, payload: dict = Body(...), db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    """Модерация: выключить/включить видео или звук у одного участника или у всех."""
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)  # только ведущий/организатор
    kind = payload.get("kind")  # audio | video
    muted = bool(payload.get("muted"))
    target = payload.get("identity")  # 'all' или конкретный identity
    src = "MICROPHONE" if kind == "audio" else "CAMERA"
    typ = "AUDIO" if kind == "audio" else "VIDEO"
    try:
        parts = _room_service("ListParticipants", {"room": c.room}, c.room).get("participants", [])
        for p in parts:
            if target != "all" and p.get("identity") != target:
                continue
            for t in p.get("tracks", []):
                if t.get("source") == src or (t.get("source") in (None, "UNKNOWN") and t.get("type") == typ):
                    _room_service("MutePublishedTrack",
                                  {"room": c.room, "identity": p["identity"], "track_sid": t["sid"], "muted": muted}, c.room)
    except Exception as e:  # noqa: BLE001
        raise HTTPException(status_code=502, detail=f"Ошибка модерации: {e}")
    return {"ok": True}


@router.post("/livekit-webhook", include_in_schema=False)
async def livekit_webhook(request: Request):
    """LiveKit присылает события комнаты. При закрытии комнаты — помечаем встречу завершённой."""
    body = await request.body()
    auth = request.headers.get("Authorization", "").removeprefix("Bearer ").strip()
    try:
        jwt.decode(auth, settings.LIVEKIT_API_SECRET, algorithms=["HS256"], options={"verify_aud": False})
    except Exception:  # noqa: BLE001
        raise HTTPException(status_code=401, detail="bad signature")
    try:
        data = json.loads(body.decode())
    except Exception:  # noqa: BLE001
        return {"ok": True}
    event = data.get("event")
    room = (data.get("room") or {}).get("name")
    if event == "room_finished" and room:
        with SessionLocal() as db:
            c = db.query(Conference).filter(Conference.room == room, Conference.status == "live").first()
            if c:
                c.status = "ended"
                c.ended_at = func.now()
                db.commit()
    return {"ok": True}
