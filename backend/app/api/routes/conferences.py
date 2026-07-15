import json
import secrets
import time
import urllib.request
import uuid

import jwt
from fastapi import APIRouter, Body, Depends, Header, HTTPException, Query, Request, status
from sqlalchemy import case, func
from sqlalchemy.orm import Session, joinedload

from app.api.deps import require_cap
from app.core.capabilities import has_cap, user_is_superadmin
from app.core.config import settings
from app.core.database import SessionLocal, get_db
from app.models import Conference, ConferenceBan, ConferenceRecording, User
from app.schemas.conference import ConferenceCreate, ConferenceOut, ConferenceParticipant, ConferenceUpdate, JoinOut

router = APIRouter(prefix="/conferences", tags=["conferences"])

_CODE_ALPHABET = "abcdefghijkmnpqrstuvwxyzACDEFGHJKLMNPQRSTUVWXYZ23456789"  # без похожих 0O1lI


def _gen_code(db: Session) -> str:
    for _ in range(20):
        code = "".join(secrets.choice(_CODE_ALPHABET) for _ in range(7))
        if not db.query(Conference).filter(Conference.code == code).first():
            return code
    return uuid.uuid4().hex[:10]


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


# ── запись (LiveKit Egress) ──
def _egress_token() -> str:
    now = int(time.time())
    return jwt.encode(
        {"iss": settings.LIVEKIT_API_KEY, "nbf": now - 5, "exp": now + 300, "video": {"roomRecord": True}},
        settings.LIVEKIT_API_SECRET, algorithm="HS256",
    )


def _egress_service(method: str, body: dict) -> dict:
    url = f"{settings.LIVEKIT_API_URL}/twirp/livekit.Egress/{method}"
    req = urllib.request.Request(
        url, data=json.dumps(body).encode(),
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {_egress_token()}"},
    )
    with urllib.request.urlopen(req, timeout=10) as r:
        return json.loads(r.read().decode() or "{}")


_REC_RES = {480: (854, 1200), 720: (1280, 2000), 1080: (1920, 3500)}


def _start_egress(room: str, filename: str, height: int = 720) -> str:
    """Запустить запись комнаты в MP4. Возвращает egress_id."""
    w, br = _REC_RES.get(height, _REC_RES[720])
    body = {
        "roomName": room,
        "layout": "grid",
        "fileOutputs": [{"fileType": "MP4", "filepath": f"/out/{filename}"}],
        "advanced": {"width": w, "height": height, "framerate": 20, "videoBitrate": br, "audioBitrate": 128},
    }
    info = _egress_service("StartRoomCompositeEgress", body)
    return info.get("egressId") or info.get("egress_id") or ""


def _stop_egress(egress_id: str):
    try:
        _egress_service("StopEgress", {"egressId": egress_id})
    except Exception:  # noqa: BLE001
        pass


def _mp4_duration_ms(path: str) -> int:
    """Длительность MP4 из атома mvhd (без внешних утилит)."""
    import os
    import struct
    try:
        total = os.path.getsize(path)
        with open(path, "rb") as f:
            def find_box(end, target):
                while f.tell() < end:
                    hdr = f.read(8)
                    if len(hdr) < 8:
                        return None
                    size, typ = struct.unpack(">I4s", hdr)
                    start = f.tell() - 8
                    if size == 1:
                        size = struct.unpack(">Q", f.read(8))[0]
                    elif size == 0:
                        size = end - start
                    if typ == target:
                        return (start, size)
                    f.seek(start + size)
                return None
            moov = find_box(total, b"moov")
            if not moov:
                return 0
            f.seek(moov[0] + 8)
            mvhd = find_box(moov[0] + moov[1], b"mvhd")
            if not mvhd:
                return 0
            f.seek(mvhd[0] + 8)
            version = f.read(1)[0]
            f.read(3)
            if version == 1:
                f.read(16); timescale = struct.unpack(">I", f.read(4))[0]; duration = struct.unpack(">Q", f.read(8))[0]
            else:
                f.read(8); timescale = struct.unpack(">I", f.read(4))[0]; duration = struct.unpack(">I", f.read(4))[0]
            return int(duration * 1000 / timescale) if timescale else 0
    except Exception:  # noqa: BLE001
        return 0


def _probe_file(rec: ConferenceRecording):
    """Уточнить размер и длительность из самого файла (egress не всегда отдаёт корректно)."""
    import os
    if not rec.filename:
        return
    path = os.path.join(settings.RECORDINGS_DIR, os.path.basename(rec.filename))
    if os.path.isfile(path):
        try:
            rec.size_bytes = os.path.getsize(path)
        except OSError:
            pass
        d = _mp4_duration_ms(path)
        if d:
            rec.duration_ms = d


def _apply_egress_info(rec: ConferenceRecording, info: dict) -> bool:
    """Обновить запись по данным egress. True — если статус изменился (завершилась)."""
    st = info.get("status")
    done_states = ("EGRESS_COMPLETE", "EGRESS_ENDING", 3)
    fail_states = ("EGRESS_FAILED", "EGRESS_ABORTED", "EGRESS_LIMIT_REACHED", 4, 5)
    files = info.get("fileResults") or info.get("file_results") or []
    if not files and info.get("file"):
        files = [info["file"]]
    if st in done_states and files and (files[0].get("filename") or files[0].get("location")):
        f = files[0]
        rec.status = "done"
        rec.filename = (f.get("filename") or "").split("/")[-1] or rec.filename
        try:
            rec.duration_ms = int(int(f.get("duration") or 0) / 1_000_000)  # ns → ms
        except (TypeError, ValueError):
            pass
        try:
            rec.size_bytes = int(f.get("size") or 0)
        except (TypeError, ValueError):
            pass
        _probe_file(rec)  # уточнить длительность/размер из самого файла (egress не всегда корректен)
        return True
    if st in fail_states:
        rec.status = "failed"
        return True
    return False


def _reconcile_recording(db: Session, rec: ConferenceRecording):
    """Подтянуть финальный статус записи из egress (если вебхук не пришёл)."""
    if rec.status != "active" or not rec.egress_id:
        return
    try:
        r = _egress_service("ListEgress", {"egressId": rec.egress_id})
        items = r.get("items") or []
    except Exception:  # noqa: BLE001
        return
    if items:
        from sqlalchemy import func as _f
        if _apply_egress_info(rec, items[0]):
            rec.ended_at = _f.now()
            db.commit()


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


def _out(c: Conference, user: User, elevated: bool, parts: list | None = None) -> ConferenceOut:
    return ConferenceOut(
        id=c.id, title=c.title, description=c.description, mode=c.mode, status=c.status, room=c.room, code=c.code,
        mic_allowed=c.mic_allowed, cam_allowed=c.cam_allowed, screen_allowed=c.screen_allowed, guests_allowed=c.guests_allowed,
        auto_record=c.auto_record,
        host_id=c.host_id, host_name=c.host.full_name if c.host else None,
        can_host=(c.host_id == user.id or elevated),
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
    elevated = user_is_superadmin(db, user)
    order = case((Conference.status == "live", 0), (Conference.status == "scheduled", 1), else_=2)
    rows = (
        db.query(Conference).options(joinedload(Conference.host))
        .order_by(order, Conference.scheduled_at.is_(None), Conference.scheduled_at.asc(), Conference.created_at.desc())
        .all()
    )
    return [_out(c, user, elevated, _live_participants(db, c) if c.status == "live" else None) for c in rows]


@router.post("", response_model=ConferenceOut, status_code=status.HTTP_201_CREATED)
def create_conference(payload: ConferenceCreate, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.host"))):
    title = (payload.title or "").strip()
    if not title:
        raise HTTPException(status_code=400, detail="Нужно название конференции")
    mode = payload.mode if payload.mode in ("interactive", "broadcast") else "interactive"
    host_id = _valid_host(db, payload.host_id) or user.id  # модератор — по умолчанию создатель
    c = Conference(
        title=title[:255], description=(payload.description or "").strip() or None,
        mode=mode, room=f"conf_{uuid.uuid4().hex[:20]}", code=_gen_code(db), host_id=host_id,
        scheduled_at=payload.scheduled_at, status="scheduled",
        mic_allowed=bool(payload.mic_allowed), cam_allowed=bool(payload.cam_allowed),
        screen_allowed=bool(payload.screen_allowed), guests_allowed=bool(payload.guests_allowed),
        auto_record=bool(payload.auto_record),
    )
    db.add(c)
    db.commit()
    db.refresh(c)
    return _out(c, user, True)


def _editable(db: Session, user: User, c: Conference):
    if c.host_id != user.id and not user_is_superadmin(db, user):
        raise HTTPException(status_code=403, detail="Управлять может только модератор конференции")


def _valid_host(db: Session, host_id: int | None) -> int | None:
    """Вернуть host_id, если это активный пользователь с правом вести конференции."""
    if not host_id:
        return None
    u = db.get(User, host_id)
    if u and u.is_active and has_cap(db, u, "conference.host"):
        return u.id
    return None


@router.get("/by-code/{code}")
def resolve_code(code: str, db: Session = Depends(get_db)):
    """Публично: короткий код → куда вести (id конференции, комната, разрешён ли гость)."""
    c = db.query(Conference).filter(Conference.code == code).first()
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    return {"id": c.id, "room": c.room, "guests_allowed": bool(c.guests_allowed), "title": c.title}


@router.get("/moderators")
def list_moderators(db: Session = Depends(get_db), user: User = Depends(require_cap("conference.host"))):
    """Пользователи, которых можно назначить модератором конференции (право conference.host)."""
    users = db.query(User).filter(User.is_active.is_(True)).order_by(User.full_name).all()
    return {"moderators": [{"id": u.id, "name": u.full_name} for u in users if has_cap(db, u, "conference.host")]}


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
    if payload.auto_record is not None:
        c.auto_record = bool(payload.auto_record)
    if payload.host_id is not None:
        vh = _valid_host(db, payload.host_id)
        if vh:
            c.host_id = vh
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
    return _out(c, user, user_is_superadmin(db, user))


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
    is_host = c.host_id == user.id or user_is_superadmin(db, user)
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
    # авто-запись при старте встречи ведущим
    if is_host and c.auto_record and _recording_on(db):
        active = db.query(ConferenceRecording).filter(ConferenceRecording.conference_id == c.id, ConferenceRecording.status == "active").first()
        if not active:
            try:
                fn = f"conf{c.id}-{uuid.uuid4().hex[:12]}.mp4"
                eid = _start_egress(c.room, fn, _rec_height(db))
                if eid:
                    db.add(ConferenceRecording(conference_id=c.id, egress_id=eid, filename=fn, status="active"))
                    db.commit()
            except Exception:  # noqa: BLE001
                pass
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
    return JoinOut(url=settings.LIVEKIT_URL, token=token, room=c.room, code=c.code, mode=c.mode, title=c.title,
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
    return JoinOut(url=settings.LIVEKIT_URL, token=token, room=c.room, code=c.code, mode=c.mode, title=c.title,
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


def _rec_out(r: ConferenceRecording, conf_title: str | None = None, can_edit: bool = False) -> dict:
    return {
        "id": r.id, "conference_id": r.conference_id, "conference_title": conf_title,
        "title": r.title or conf_title or "Запись", "description": r.description,
        "status": r.status, "duration_ms": r.duration_ms or 0, "size_bytes": r.size_bytes or 0,
        "started_at": r.started_at.isoformat() if r.started_at else None,
        "ended_at": r.ended_at.isoformat() if r.ended_at else None,
        "can_edit": can_edit,
        "url": f"{settings.API_PREFIX}/conferences/recordings/{r.id}/file" if (r.status == "done" and r.filename) else None,
    }


def _egress_configured() -> bool:
    return bool(settings.LIVEKIT_API_KEY and settings.LIVEKIT_API_SECRET)


def _recording_on(db: Session) -> bool:
    from app.api.routes.settings import get_int_setting
    return _egress_configured() and bool(get_int_setting(db, "recording_enabled", 1))


def _rec_height(db: Session) -> int:
    from app.api.routes.settings import get_int_setting
    h = get_int_setting(db, "recording_height", 720)
    return h if h in (480, 720, 1080) else 720


@router.get("/{conf_id}/record")
def record_status(conf_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    active = db.query(ConferenceRecording).filter(ConferenceRecording.conference_id == conf_id, ConferenceRecording.status == "active").first()
    if active:
        _reconcile_recording(db, active)  # вдруг уже завершилась
        active = db.query(ConferenceRecording).filter(ConferenceRecording.conference_id == conf_id, ConferenceRecording.status == "active").first()
    return {"recording": bool(active), "enabled": _recording_on(db)}


@router.post("/{conf_id}/record/start")
def record_start(conf_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    if not _recording_on(db):
        raise HTTPException(status_code=503, detail="Запись отключена в настройках")
    active = db.query(ConferenceRecording).filter(ConferenceRecording.conference_id == c.id, ConferenceRecording.status == "active").first()
    if active:
        return {"recording": True}
    filename = f"conf{c.id}-{uuid.uuid4().hex[:12]}.mp4"
    try:
        egress_id = _start_egress(c.room, filename, _rec_height(db))
    except Exception as e:  # noqa: BLE001
        raise HTTPException(status_code=502, detail=f"Не удалось начать запись: {e}")
    if not egress_id:
        raise HTTPException(status_code=502, detail="Egress не вернул id")
    db.add(ConferenceRecording(conference_id=c.id, egress_id=egress_id, filename=filename, status="active"))
    db.commit()
    return {"recording": True}


@router.post("/{conf_id}/record/stop")
def record_stop(conf_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    c = db.get(Conference, conf_id)
    if not c:
        raise HTTPException(status_code=404, detail="Конференция не найдена")
    _editable(db, user, c)
    for rec in db.query(ConferenceRecording).filter(ConferenceRecording.conference_id == c.id, ConferenceRecording.status == "active").all():
        if rec.egress_id:
            _stop_egress(rec.egress_id)  # финализируется вебхуком egress_ended
    return {"recording": False}


@router.get("/recordings")
def list_recordings(db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    """Архив записей — готовые записи всех конференций, свежие сверху."""
    # подтянуть зависшие «active» (если вебхук egress не пришёл)
    for act in db.query(ConferenceRecording).filter(ConferenceRecording.status == "active").all():
        _reconcile_recording(db, act)
    rows = (
        db.query(ConferenceRecording).options(joinedload(ConferenceRecording.conference))
        .filter(ConferenceRecording.status == "done", ConferenceRecording.filename.isnot(None))
        .order_by(ConferenceRecording.started_at.desc()).all()
    )
    # уточнить длительность/размер у записей, где они пустые (egress вернул 0)
    dirty = False
    for r in rows:
        if not r.duration_ms or not r.size_bytes:
            _probe_file(r); dirty = True
    if dirty:
        db.commit()
    elevated = user_is_superadmin(db, user)
    def _ce(r):
        return bool(r.conference and (r.conference.host_id == user.id or elevated))
    return {"recordings": [_rec_out(r, r.conference.title if r.conference else None, _ce(r)) for r in rows]}


@router.patch("/recordings/{rec_id}")
def update_recording(rec_id: int, payload: dict = Body(...), db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    """Модератор может задать своё название и описание записи."""
    r = db.get(ConferenceRecording, rec_id)
    if not r:
        raise HTTPException(status_code=404, detail="Запись не найдена")
    c = db.get(Conference, r.conference_id)
    if not c or (c.host_id != user.id and not user_is_superadmin(db, user)):
        raise HTTPException(status_code=403, detail="Менять запись может ведущий или модератор")
    if "title" in payload:
        t = (payload.get("title") or "").strip()
        r.title = t[:255] or None
    if "description" in payload:
        d = (payload.get("description") or "").strip()
        r.description = d or None
    db.commit()
    db.refresh(r)
    return _rec_out(r, c.title, True)


@router.delete("/recordings/{rec_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_recording(rec_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("conference.view"))):
    """Удалить запись (файл с сервера и запись из архива)."""
    r = db.get(ConferenceRecording, rec_id)
    if not r:
        raise HTTPException(status_code=404, detail="Запись не найдена")
    c = db.get(Conference, r.conference_id)
    if not c or (c.host_id != user.id and not user_is_superadmin(db, user)):
        raise HTTPException(status_code=403, detail="Удалить запись может ведущий или модератор")
    if r.status == "active" and r.egress_id:
        _stop_egress(r.egress_id)
    if r.filename:
        import os
        path = os.path.join(settings.RECORDINGS_DIR, os.path.basename(r.filename))
        try:
            os.remove(path)
        except OSError:
            pass
    db.delete(r)
    db.commit()


@router.get("/recordings/{rec_id}/file")
def recording_file(rec_id: int, token: str | None = Query(None), authorization: str | None = Header(None), db: Session = Depends(get_db)):
    """Отдать файл записи (с поддержкой Range для <video>). Авторизация — заголовком или ?token=."""
    import os
    import jwt
    from fastapi.responses import FileResponse
    from app.core.security import decode_access_token
    raw = token or (authorization or "").removeprefix("Bearer ").strip()
    try:
        email = decode_access_token(raw).get("sub")
        u = db.query(User).filter(User.email == email).first() if email else None
    except jwt.PyJWTError:
        u = None
    if not u or not u.is_active:
        raise HTTPException(status_code=401, detail="Не авторизован")
    if not has_cap(db, u, "conference.view"):
        raise HTTPException(status_code=403, detail="Недостаточно прав")
    r = db.get(ConferenceRecording, rec_id)
    if not r or not r.filename or r.status != "done":
        raise HTTPException(status_code=404, detail="Запись не найдена")
    path = os.path.join(settings.RECORDINGS_DIR, os.path.basename(r.filename))
    if not os.path.isfile(path):
        raise HTTPException(status_code=404, detail="Файл записи не найден")
    return FileResponse(path, media_type="video/mp4", filename=os.path.basename(path))


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
                # остановить активную запись (room composite сам завершится, но подстрахуемся)
                for rec in db.query(ConferenceRecording).filter(ConferenceRecording.conference_id == c.id, ConferenceRecording.status == "active").all():
                    if rec.egress_id:
                        _stop_egress(rec.egress_id)
    elif event in ("egress_ended", "egress_updated") and data.get("egressInfo"):
        info = data["egressInfo"]
        eid = info.get("egressId") or info.get("egress_id")
        st = info.get("status")  # EGRESS_COMPLETE / EGRESS_FAILED / EGRESS_ABORTED / ...
        with SessionLocal() as db:
            rec = db.query(ConferenceRecording).filter(ConferenceRecording.egress_id == eid).first()
            if rec and rec.status == "active":
                files = info.get("fileResults") or info.get("file_results") or []
                if not files and info.get("file"):
                    files = [info["file"]]
                if event == "egress_ended":
                    if st in ("EGRESS_COMPLETE", 3) and files:
                        f = files[0]
                        rec.status = "done"
                        rec.filename = (f.get("filename") or rec.filename or "").split("/")[-1]
                        try:
                            rec.duration_ms = int(int(f.get("duration") or 0) / 1_000_000)  # ns → ms
                        except (TypeError, ValueError):
                            pass
                        try:
                            rec.size_bytes = int(f.get("size") or 0)
                        except (TypeError, ValueError):
                            pass
                    else:
                        rec.status = "failed"
                    rec.ended_at = func.now()
                    db.commit()
    return {"ok": True}
