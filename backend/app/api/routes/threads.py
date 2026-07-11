from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy import func
from sqlalchemy.orm import Session, joinedload

from app.api.deps import get_current_user
from app.core.database import get_db
from app.core.enums import Role, ThreadKind
from app.models import Disciple, MessageLike, Thread, ThreadMessage, ThreadRead, User
from app.schemas.thread import MessageCreate, MessageOut, ThreadCreate, ThreadListItem, ThreadOut

router = APIRouter(prefix="/threads", tags=["threads"])


def _accessible(db: Session, user: User):
    """Ветки, доступные пользователю по роли."""
    q = db.query(Thread).join(Disciple, Thread.disciple_id == Disciple.id)
    if user.role == Role.guru:
        return q
    if user.role == Role.curator:
        return q.filter(Thread.kind == ThreadKind.report, Disciple.mentor_id == user.id)
    if user.role == Role.student:
        return q.filter(Thread.disciple_id == (user.disciple_id or -1))
    return q.filter(Thread.id < 0)  # секретарь и прочие — нет доступа


def _msg_out(m: ThreadMessage, user_id: int) -> MessageOut:
    return MessageOut(
        id=m.id, author_id=m.author_id,
        author_name=m.author.full_name if m.author else None,
        body=m.body, created_at=m.created_at,
        likes=len(m.likes), liked=any(l.user_id == user_id for l in m.likes),
    )


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
    db.commit()
    db.refresh(msg)
    _mark_read(db, user, t.id)  # автор только что видел ветку
    return _msg_out(msg, user.id)


@router.post("/{thread_id}/messages/{message_id}/like")
def toggle_like(thread_id: int, message_id: int, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    if user.role not in (Role.guru, Role.curator):
        raise HTTPException(status_code=403, detail="Лайкать может гуру или наставник")
    _get_accessible_thread(db, user, thread_id)  # проверка доступа к ветке
    msg = db.get(ThreadMessage, message_id)
    if not msg or msg.thread_id != thread_id:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    existing = db.query(MessageLike).filter(
        MessageLike.message_id == message_id, MessageLike.user_id == user.id
    ).first()
    if existing:
        db.delete(existing)
        liked = False
    else:
        db.add(MessageLike(message_id=message_id, user_id=user.id))
        liked = True
    db.commit()
    likes = db.query(MessageLike).filter(MessageLike.message_id == message_id).count()
    return {"likes": likes, "liked": liked}
