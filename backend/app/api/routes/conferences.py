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
from app.models import Conference, ConferenceBan, User
from app.schemas.conference import ConferenceCreate, ConferenceOut, ConferenceParticipant, ConferenceUpdate, JoinOut

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


def _close_room(room: str):
    """Закрыть комнату LiveKit — всех участников отключит (при завершении встречи)."""
    if not (settings.LIVEKIT_API_KEY and settings.LIVEKIT_API_SECRET):
        return
    try:
        _room_service("DeleteRoom", {"room": room}, room)
    except Exception:  # noqa: BLE001
        pass


def _mint_token(identity: str, name: str, room: str, can_publish: bool, sources: list | None = None, ttl: int = 6 * 3600) -> str:
    """LiveKit access token (JWT, HS256). sources ограничивает, что можно публиковать."""
    now = int(time.time())
    video = {
        "room": room, "roomJoin": True, "canPublish": can_publish,
        "canSubscribe": True, "canPublishData": True,
    }
    if sources is not None:
        video["canPublishSources"] = sources
    claims = {"iss": settings.LIVEKIT_API_KEY, "sub": identity, "name": name, "nbf": now - 5, "exp": now + ttl, "video": video}
    return jwt.encode(claims, settings.LIVEKIT_API_SECRET, algorithm="HS256")


def _out(c: Conference, user: User, is_host_cap: bool, parts: list | None = None) -> ConferenceOut:
    return ConferenceOut(
        id=c.id, title=c.title, description=c.description, mode=c.mode, status=c.status, room=c.room,
        mic_allowed=c.mic_allowed, cam_allowed=c.cam_allowed, screen_allowed=c.screen_allowed, guests_allowed=c.guests_allowed,
        host_id=c.host_id, host_name=c.host.full_name if c.host else None,
        can_host=(c.host_id == user.id or is_host_cap),
        scheduled_at=c.scheduled_at, started_at=c.started_at, ended_at=c.ended_at, created_at=c.created_at,
        participant_count=len(parts) if parts is not None else 0,
        participants=(parts or [])[:6],
    )


def _live_participants(db: Session, c: Conference) -> list[ConferenceParticipant]:
    """Список подключённых к живой встрече (для карточки): имя + аватар (по возможности)."""
    if not (settings.LIVEKIT_API_KEY and settings.LIVEKIT_API_SECRET):
        return []
    try:
        raw = _room_service("ListParticipants", {"room": c.room}, c.room).get("participants", [])
    except Exception:  # noqa: BLE001
        return []
    # аватары для авторизованных участников (identity = "u{user_id}")
    uids = [int(p["identity"][1:]) for p in raw if (p.get("identity") or "").startswith("u") and p["identity"][1:].isdigit()]
    avatars = {}
    if uids:
        for u in db.query(User).filter(User.id.in_(uids)).all():
            avatars[f"u{u.id}"] = u.avatar_url
    out = []
    for p in raw:
        ident = p.get("identity") or ""
        out.append(ConferenceParticipant(name=p.get("name") or "Гость", avatar_url=avatars.get(ident)))
    return out


@router.get("", response_model=list[ConferenceOut])
def list_conferences(db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    is_host_cap = has_cap(db, user, "conference.host")
    order = case((Conference.status == "live", 0), (Conference.status == "scheduled", 1), else_=2)
    rows = (
        db.query(Conference).options(joinedload(Conference.host))
        .order_by(order, Conference.scheduled_at.is_(None), Conference.scheduled_at.asc(), Conference.created_at.desc())
        .all()
    )
    return [_out(c, user, is_host_cap, _live_participants(db, c) if c.status == "live" else None) for c in rows]


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
        mic_allowed=bool(payload.mic_allowed), cam_allowed=bool(payload.cam_allowed),
        screen_allowed=bool(payload.screen_allowed), guests_allowed=bool(payload.guests_allowed),
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
    if payload.mode in ("interactive", "broadcast"):
        c.mode = payload.mode
    if payload.mic_allowed is not None:
        c.mic_allowed = bool(payload.mic_allowed)
    if payload.cam_allowed is not None:
        c.cam_allowed = bool(payload.cam_allowed)
    if payload.screen_allowed is not None:
        c.screen_allowed = bool(payload.screen_allowed)
    if payload.guests_allowed is not None:
        c.guests_allowed = bool(payload.guests_allowed)
    if payload.status in ("scheduled", "live", "ended"):
        c.status = payload.status
        from sqlalchemy import func as _f
        if payload.status == "live" and not c.started_at:
            c.started_at = _f.now()
        if payload.status == "ended":
            c.ended_at = _f.now()
    db.commit()
    db.refresh(c)
    if payload.status == "ended":
        _close_room(c.room)  # отключить всех оставшихся участников
    return _out(c, user, has_cap(db, user, "conference.host"))


@router.delete("/{conf_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_conference(conf_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    room = c.room
    db.delete(c)
    db.commit()
    _close_room(room)  # если удаляют идущую встречу — отключить участников


@router.post("/{conf_id}/join", response_model=JoinOut)
def join_conference(conf_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    if not (settings.LIVEKIT_API_KEY and settings.LIVEKIT_API_SECRET and settings.LIVEKIT_URL):
        raise HTTPException(status_code=503, detail="Видеосервер не настроен")
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    is_host = c.host_id == user.id or has_cap(db, user, "conference.host")
    identity = f"u{user.id}"
    if not is_host and db.query(ConferenceBan).filter(ConferenceBan.conference_id == c.id, ConferenceBan.identity == identity).first():
        raise HTTPException(status_code=403, detail="Вы удалены из этой встречи ведущим")
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
    # источники публикации: ведущий — всё; участник — по флагам конференции (трансляция — только ведущий)
    if is_host:
        sources = None
    else:
        sources = []
        if c.mode != "broadcast":
            if c.mic_allowed:
                sources.append("MICROPHONE")
            if c.cam_allowed:
                sources.append("CAMERA")
            if c.screen_allowed:
                sources.append("SCREEN_SHARE")
        can_publish = len(sources) > 0
    token = _mint_token(identity, user.full_name or "Гость", c.room, can_publish, sources)
    return JoinOut(url=settings.LIVEKIT_URL, token=token, room=c.room, mode=c.mode, title=c.title,
                   can_publish=can_publish, is_host=is_host, identity=identity,
                   mic_allowed=c.mic_allowed, cam_allowed=c.cam_allowed, screen_allowed=c.screen_allowed)


@router.post("/guest/{room}", response_model=JoinOut)
def guest_join(room: str, payload: dict = Body(...), db: Session = Depends(get_db)):
    """Гостевой вход по ссылке (без авторизации), если разрешён создателем."""
    if not (settings.LIVEKIT_API_KEY and settings.LIVEKIT_API_SECRET and settings.LIVEKIT_URL):
        raise HTTPException(status_code=503, detail="Видеосервер не настроен")
    c = db.query(Conference).filter(Conference.room == room).first()
    if not c or not c.guests_allowed:
        raise HTTPException(status_code=404, detail="Конференция недоступна для гостей")
    name = (payload.get("name") or "").strip()[:60] or "Гость"
    if c.status == "ended":
        c.status = "live"
        c.ended_at = None
        if not c.started_at:
            c.started_at = func.now()
        db.commit()
    if c.mode == "broadcast":
        sources, can_publish = [], False
    else:
        sources = []
        if c.mic_allowed:
            sources.append("MICROPHONE")
        if c.cam_allowed:
            sources.append("CAMERA")
        if c.screen_allowed:
            sources.append("SCREEN_SHARE")
        can_publish = len(sources) > 0
    identity = f"g_{uuid.uuid4().hex[:12]}"
    token = _mint_token(identity, name, c.room, can_publish, sources)
    return JoinOut(url=settings.LIVEKIT_URL, token=token, room=c.room, mode=c.mode, title=c.title,
                   can_publish=can_publish, is_host=False, identity=identity,
                   mic_allowed=c.mic_allowed, cam_allowed=c.cam_allowed, screen_allowed=c.screen_allowed)


_SRC = {"audio": ["MICROPHONE"], "video": ["CAMERA"], "screen": ["SCREEN_SHARE"]}
_MUTE_SRC = {"audio": "MICROPHONE", "video": "CAMERA", "screen": "SCREEN_SHARE"}


def _apply_perm(room: str, p: dict, kind: str, allow: bool):
    """Выдать/забрать право публикации источника у участника (+ заглушить при запрете)."""
    perm = p.get("permission") or {}
    cur = set(perm.get("canPublishSources") or perm.get("can_publish_sources") or [])
    if not cur and (perm.get("canPublish") or perm.get("can_publish")):
        cur = {"CAMERA", "MICROPHONE", "SCREEN_SHARE"}
    for s in _SRC[kind]:
        cur.add(s) if allow else cur.discard(s)
    _room_service("UpdateParticipant", {
        "room": room, "identity": p["identity"],
        "permission": {"canPublish": len(cur) > 0, "canPublishSources": sorted(cur), "canSubscribe": True, "canPublishData": True},
    }, room)
    if not allow:
        src = _MUTE_SRC[kind]
        typ = "AUDIO" if kind == "audio" else "VIDEO"
        for t in p.get("tracks", []):
            if t.get("source") == src or (t.get("source") in (None, "UNKNOWN") and t.get("type") == typ):
                try:
                    _room_service("MutePublishedTrack", {"room": room, "identity": p["identity"], "track_sid": t["sid"], "muted": True}, room)
                except Exception:  # noqa: BLE001
                    pass


@router.post("/{conf_id}/permit")
def moderate_permit(conf_id: int, payload: dict = Body(...), db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    """Разрешить/запретить участникам звук или видео: одному, всем, или всем кроме одного."""
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    kind = payload.get("kind")
    if kind not in ("audio", "video", "screen"):
        raise HTTPException(status_code=400, detail="kind: audio|video|screen")
    allow = bool(payload.get("allow"))
    target = payload.get("identity")  # 'all' | identity
    exc = payload.get("except")
    if target == "all":  # меняем и дефолт для новых участников
        if kind == "audio":
            c.mic_allowed = allow
        elif kind == "video":
            c.cam_allowed = allow
        else:
            c.screen_allowed = allow
        db.commit()
    host_ident = f"u{c.host_id}" if c.host_id else None
    try:
        parts = _room_service("ListParticipants", {"room": c.room}, c.room).get("participants", [])
        for p in parts:
            ident = p.get("identity")
            if ident == host_ident:
                continue
            if target != "all" and ident != target:
                continue
            if target == "all" and exc and ident == exc:
                _apply_perm(c.room, p, kind, True)  # исключение — оставить включённым
                continue
            _apply_perm(c.room, p, kind, allow)
    except Exception as e:  # noqa: BLE001
        raise HTTPException(status_code=502, detail=f"Ошибка модерации: {e}")
    return {"ok": True}


def _bans_out(db: Session, conf_id: int) -> list[dict]:
    rows = db.query(ConferenceBan).filter(ConferenceBan.conference_id == conf_id).order_by(ConferenceBan.created_at.desc()).all()
    return [{"identity": b.identity, "name": b.name or "Участник"} for b in rows]


@router.get("/{conf_id}/bans")
def list_bans(conf_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    return {"bans": _bans_out(db, conf_id)}


@router.post("/{conf_id}/kick")
def kick_participant(conf_id: int, payload: dict = Body(...), db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    """Удалить участника из конференции и добавить в список заблокированных."""
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    identity = (payload.get("identity") or "").strip()
    name = (payload.get("name") or "").strip()[:120] or "Участник"
    if not identity:
        raise HTTPException(status_code=400, detail="Не указан участник")
    if identity == (f"u{c.host_id}" if c.host_id else None):
        raise HTTPException(status_code=400, detail="Нельзя удалить ведущего")
    # добавить в бан (для авторизованных identity=u{id} — не сможет вернуться; гость получит новый id)
    if not db.query(ConferenceBan).filter(ConferenceBan.conference_id == c.id, ConferenceBan.identity == identity).first():
        db.add(ConferenceBan(conference_id=c.id, identity=identity, name=name))
        db.commit()
    # отключить прямо сейчас
    try:
        _room_service("RemoveParticipant", {"room": c.room, "identity": identity}, c.room)
    except Exception:  # noqa: BLE001
        pass
    return {"ok": True, "bans": _bans_out(db, c.id)}


@router.delete("/{conf_id}/bans/{identity}")
def unban_participant(conf_id: int, identity: str, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    """Убрать участника из списка заблокированных — снова сможет зайти."""
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    row = db.query(ConferenceBan).filter(ConferenceBan.conference_id == c.id, ConferenceBan.identity == identity).first()
    if row:
        db.delete(row)
        db.commit()
    return {"ok": True, "bans": _bans_out(db, c.id)}


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
