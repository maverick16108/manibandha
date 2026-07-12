from datetime import datetime, timedelta, timezone

from fastapi import APIRouter, Body, Depends, HTTPException, Query, status
from sqlalchemy import func
from sqlalchemy.orm import Session, joinedload

from app.api.deps import get_current_user
from app.core.database import get_db
from app.core.enums import Role, ThreadKind
from app.models import Disciple, MessageLike, Thread, ThreadMessage, ThreadRead, User
from app.schemas.thread import MessageCreate, MessageOut, ThreadCreate, ThreadListItem, ThreadOut

router = APIRouter(prefix="/threads", tags=["threads"])


def _accessible(db: Session, user: User):
    """Ветки, доступные пользователю по правам-действиям."""
    from sqlalchemy import and_, or_
    from app.core.capabilities import user_capabilities

    caps = user_capabilities(db, user)
    own = user.disciple_id or -1
    q = db.query(Thread).join(Disciple, Thread.disciple_id == Disciple.id)

    conds = [Thread.disciple_id == own]  # свои ветки (вопросы/отчёты/чат апрува)
    if "questions.answer" in caps or "questions.view_all" in caps:
        conds.append(Thread.kind == ThreadKind.question)
    if "reports.read_all" in caps:
        if "disciples.view_all" in caps:
            conds.append(Thread.kind == ThreadKind.report)
        else:  # наставник — только отчёты закреплённых
            conds.append(and_(Thread.kind == ThreadKind.report, Disciple.mentor_id == own))
    if "disciples.approve" in caps:
        conds.append(Thread.kind == ThreadKind.approval)
    return q.filter(or_(*conds))


# 6 доступных реакций
REACTIONS = ["❤️", "👍", "🙏", "🔥", "😂", "🎉"]


def _reactions_of(m: ThreadMessage, user_id: int) -> list[dict]:
    from collections import Counter
    counts = Counter(l.emoji for l in m.likes)
    mine = next((l.emoji for l in m.likes if l.user_id == user_id), None)
    ordered = sorted(counts.items(), key=lambda kv: (-kv[1], REACTIONS.index(kv[0]) if kv[0] in REACTIONS else 99))
    return [{"emoji": e, "count": c, "mine": e == mine} for e, c in ordered]


def _msg_out(m: ThreadMessage, user_id: int) -> MessageOut:
    return MessageOut(
        id=m.id, author_id=m.author_id,
        author_name=m.author.full_name if m.author else None,
        body=m.body, created_at=m.created_at, edit_count=m.edit_count or 0,
        reactions=_reactions_of(m, user_id),
    )


EDIT_WINDOW = timedelta(hours=1)


def _within_edit_window(m: ThreadMessage) -> bool:
    created = m.created_at
    if created is None:
        return False
    if created.tzinfo is None:
        created = created.replace(tzinfo=timezone.utc)
    return (datetime.now(timezone.utc) - created) <= EDIT_WINDOW


async def _broadcast(thread_id: int, data: dict) -> None:
    """Разослать событие подключённым к ветке (правка/удаление в реальном времени)."""
    try:
        from app.api.routes.ws import manager
        await manager.broadcast(thread_id, data)
    except Exception:
        pass


@router.get("", response_model=list[ThreadListItem])
def list_threads(
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
    kind: ThreadKind | None = None,
    disciple_id: int | None = None,
    period: str | None = None,
):
    q = _accessible(db, user).options(joinedload(Thread.disciple), joinedload(Thread.messages))
    if kind:
        q = q.filter(Thread.kind == kind)
    if disciple_id:
        q = q.filter(Thread.disciple_id == disciple_id)
    if period:
        q = q.filter(Thread.period == period)
    rows = q.order_by(Thread.updated_at.desc()).all()
    seen = {
        r.thread_id: r.last_seen_at
        for r in db.query(ThreadRead).filter(ThreadRead.user_id == user.id).all()
    }
    out = []
    for t in rows:
        last = t.messages[-1] if t.messages else None
        ls = seen.get(t.id)
        unread = ls is None or (t.updated_at and ls and t.updated_at > ls)
        out.append(ThreadListItem(
            id=t.id, kind=t.kind, disciple_id=t.disciple_id,
            disciple_name=(t.disciple.spiritual_name or t.disciple.material_name) if t.disciple else "—",
            subject=t.subject, period=t.period, updated_at=t.updated_at,
            messages_count=len(t.messages),
            last_preview=(last.body[:120] if last else None),
            unread=bool(unread),
        ))
    return out


def _mark_read(db: Session, user: User, thread_id: int):
    r = db.query(ThreadRead).filter(ThreadRead.thread_id == thread_id, ThreadRead.user_id == user.id).first()
    if r:
        r.last_seen_at = func.now()
    else:
        db.add(ThreadRead(thread_id=thread_id, user_id=user.id))
    db.commit()


def _is_recipient(caps: set, kind: ThreadKind) -> bool:
    """Сторона-получатель ветки (кому адресованы вопросы/отчёты/заявки)."""
    if kind == ThreadKind.question:
        return "questions.answer" in caps or "questions.view_all" in caps
    if kind == ThreadKind.report:
        return "reports.read_all" in caps
    if kind == ThreadKind.approval:
        return "disciples.approve" in caps
    return False


def _mark_staff_seen(db: Session, user: User, thread: Thread):
    """Отметить, что сторона-получатель видела ветку (общий счётчик непросмотренных)."""
    from app.core.capabilities import user_capabilities
    if _is_recipient(user_capabilities(db, user), thread.kind):
        thread.staff_seen_at = func.now()


@router.get("/nav-counts")
def nav_counts(db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    """Счётчики непросмотренного для меню.

    Сторона-получатель (гуру/куратор) — общий счётчик по staff_seen_at.
    Сторона-владелец (ученик) — личный счётчик по ThreadRead (ответы на его ветки).
    Плюс неодобренные заявки для тех, кто может апрувить.
    """
    from app.core.capabilities import user_capabilities
    caps = user_capabilities(db, user)
    seen = {r.thread_id: r.last_seen_at for r in db.query(ThreadRead).filter(ThreadRead.user_id == user.id).all()}

    def count_unread(kind: ThreadKind) -> int:
        recipient = _is_recipient(caps, kind)
        threads = _accessible(db, user).filter(Thread.kind == kind).all()
        c = 0
        for t in threads:
            if recipient:
                unread = t.staff_seen_at is None or (t.updated_at and t.updated_at > t.staff_seen_at)
            else:
                ls = seen.get(t.id)
                unread = ls is None or (t.updated_at and ls and t.updated_at > ls)
            if unread:
                c += 1
        return c

    res = {
        "questions": count_unread(ThreadKind.question),
        "reports": count_unread(ThreadKind.report),
        "approvals": 0,
    }
    if "disciples.approve" in caps:
        res["approvals"] = db.query(Disciple).filter(Disciple.is_approved.is_(False)).count()
    return res


@router.get("/stats")
def thread_stats(disciple_id: int, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    """Сколько у ученика вопросов, отчётов и написанных им сообщений."""
    questions = db.query(Thread).filter(
        Thread.disciple_id == disciple_id, Thread.kind == ThreadKind.question
    ).count()
    reports = db.query(Thread).filter(
        Thread.disciple_id == disciple_id, Thread.kind == ThreadKind.report
    ).count()
    student = db.query(User).filter(User.disciple_id == disciple_id).first()
    messages = 0
    if student:
        messages = (
            db.query(ThreadMessage).join(Thread, ThreadMessage.thread_id == Thread.id)
            .filter(Thread.disciple_id == disciple_id, ThreadMessage.author_id == student.id).count()
        )
    return {"questions": questions, "reports": reports, "messages": messages}


def _get_accessible_thread(db: Session, user: User, thread_id: int) -> Thread:
    t = _accessible(db, user).filter(Thread.id == thread_id).first()
    if not t:
        raise HTTPException(status_code=404, detail="Ветка не найдена")
    return t


@router.get("/{thread_id}", response_model=ThreadOut)
def get_thread(thread_id: int, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    t = _get_accessible_thread(db, user, thread_id)
    _mark_staff_seen(db, user, t)  # общий счётчик непросмотренных для стороны-получателя
    _mark_read(db, user, t.id)
    return ThreadOut(
        id=t.id, kind=t.kind, disciple_id=t.disciple_id,
        disciple_name=(t.disciple.spiritual_name or t.disciple.material_name) if t.disciple else "—",
        subject=t.subject, period=t.period, created_at=t.created_at, updated_at=t.updated_at,
        messages=[_msg_out(m, user.id) for m in t.messages],
    )


@router.post("", response_model=ThreadOut, status_code=status.HTTP_201_CREATED)
def create_thread(payload: ThreadCreate, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    # чей это ученик
    if user.disciple_id:
        disciple_id = user.disciple_id
    elif user.role == Role.guru and payload.disciple_id:
        disciple_id = payload.disciple_id
    else:
        raise HTTPException(status_code=403, detail="Нельзя создать ветку без ученика")
    if not db.get(Disciple, disciple_id):
        raise HTTPException(status_code=404, detail="Ученик не найден")

    if payload.kind == ThreadKind.report and not payload.period:
        raise HTTPException(status_code=400, detail="Для отчёта нужен месяц (period)")

    # отчёт за месяц — один: если есть, добавляем сообщение в существующую ветку
    thread = None
    if payload.kind == ThreadKind.report:
        thread = (
            db.query(Thread)
            .filter(Thread.kind == ThreadKind.report, Thread.disciple_id == disciple_id, Thread.period == payload.period)
            .first()
        )
    if thread is None:
        thread = Thread(kind=payload.kind, disciple_id=disciple_id, subject=payload.subject, period=payload.period)
        db.add(thread)
        db.flush()
    msg = ThreadMessage(thread_id=thread.id, author_id=user.id, body=payload.body)
    db.add(msg)
    db.commit()
    db.refresh(thread)
    return get_thread(thread.id, db, user)


@router.post("/{thread_id}/messages", response_model=MessageOut, status_code=status.HTTP_201_CREATED)
def add_message(thread_id: int, payload: MessageCreate, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    t = _get_accessible_thread(db, user, thread_id)
    if not payload.body.strip():
        raise HTTPException(status_code=400, detail="Пустое сообщение")
    msg = ThreadMessage(thread_id=t.id, author_id=user.id, body=payload.body.strip())
    db.add(msg)
    t.updated_at = func.now()  # поднять ветку наверх по последней активности
    _mark_staff_seen(db, user, t)  # если пишет сторона-получатель — ветка просмотрена
    db.commit()
    db.refresh(msg)
    _mark_read(db, user, t.id)  # автор только что видел ветку
    return _msg_out(msg, user.id)


def _get_own_editable(db: Session, user: User, thread_id: int, message_id: int) -> ThreadMessage:
    _get_accessible_thread(db, user, thread_id)  # доступ к ветке
    msg = db.get(ThreadMessage, message_id)
    if not msg or msg.thread_id != thread_id:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    if msg.author_id != user.id:
        raise HTTPException(status_code=403, detail="Можно менять только свои сообщения")
    if not _within_edit_window(msg):
        raise HTTPException(status_code=403, detail="Прошёл час — сообщение больше нельзя изменить")
    return msg


@router.patch("/{thread_id}/messages/{message_id}", response_model=MessageOut)
async def edit_message(thread_id: int, message_id: int, payload: MessageCreate, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    msg = _get_own_editable(db, user, thread_id, message_id)
    body = payload.body.strip()
    if not body:
        raise HTTPException(status_code=400, detail="Пустое сообщение")
    msg.body = body
    msg.edited_at = func.now()
    msg.edit_count = (msg.edit_count or 0) + 1
    db.commit()
    db.refresh(msg)
    out = _msg_out(msg, user.id)
    await _broadcast(thread_id, {
        "type": "edit",
        "message": {"id": msg.id, "body": msg.body, "edit_count": msg.edit_count},
    })
    return out


@router.delete("/{thread_id}/messages/{message_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_message(thread_id: int, message_id: int, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    msg = _get_own_editable(db, user, thread_id, message_id)
    db.delete(msg)
    db.commit()
    await _broadcast(thread_id, {"type": "delete", "message_id": message_id})


@router.post("/{thread_id}/messages/{message_id}/react")
async def react(thread_id: int, message_id: int, payload: dict = Body(...),
                db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    """Поставить/сменить/снять реакцию-эмодзи (одна на пользователя на сообщение)."""
    emoji = (payload or {}).get("emoji")
    if emoji not in REACTIONS:
        raise HTTPException(status_code=400, detail="Недопустимая реакция")
    _get_accessible_thread(db, user, thread_id)
    msg = db.get(ThreadMessage, message_id)
    if not msg or msg.thread_id != thread_id:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    existing = db.query(MessageLike).filter(
        MessageLike.message_id == message_id, MessageLike.user_id == user.id
    ).first()
    if existing:
        if existing.emoji == emoji:
            db.delete(existing)  # тот же эмодзи — снять реакцию
        else:
            existing.emoji = emoji  # сменить на другой
    else:
        db.add(MessageLike(message_id=message_id, user_id=user.id, emoji=emoji))
    db.commit()
    db.refresh(msg)
    reactions = _reactions_of(msg, user.id)
    await _broadcast(thread_id, {"type": "react", "message_id": message_id, "reactions": reactions})
    return {"reactions": reactions}
