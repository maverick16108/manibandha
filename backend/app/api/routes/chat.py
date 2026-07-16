"""Мессенджер: чаты (личные/групповые), сообщения, sync-курсор (getDifference-стиль).

Сервер — источник истины. Отправка идемпотентна по client_uuid, порядок доставки —
глобальный seq. Клиент догоняет пропущенное через GET /chats/updates?since={pts}.
"""
import re
from datetime import datetime, timedelta, timezone

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy import and_, func, or_, select, text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session, joinedload

from app.api.deps import get_current_user
from app.core.database import get_db
from app.core.enums import ChatType, Role
from app.models import Chat, ChatMember, ChatMessage, Disciple, User
from app.schemas.chat import (
    ChatCreateIn, ChatMemberOut, ChatMessageOut, ChatOut, ChatUpdate, ContactOut,
    EditMessageIn, ReadIn, SendMessageIn, UpdatesOut,
)

router = APIRouter(prefix="/chats", tags=["chats"])

EDIT_WINDOW = timedelta(hours=24)


# ── доступ ────────────────────────────────────────────────────────────────
def _is_pending(db: Session, user: User) -> bool:
    """Незаапрувленный кандидат (только экран ожидания) — в мессенджер не пускаем."""
    if not user.disciple_id:
        return False
    d = db.get(Disciple, user.disciple_id)
    return bool(d and not d.is_approved)


def chat_user(db: Session = Depends(get_db), user: User = Depends(get_current_user)) -> User:
    if _is_pending(db, user):
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Чат станет доступен после одобрения заявки")
    return user


def _my_chat_ids(db: Session, user: User):
    return select(ChatMember.chat_id).where(ChatMember.user_id == user.id)


def _member_ids(db: Session, chat_id: int) -> list[int]:
    return [m.user_id for m in db.query(ChatMember).filter(ChatMember.chat_id == chat_id).all()]


def _require_membership(db: Session, user: User, chat_id: int) -> Chat:
    chat = db.get(Chat, chat_id)
    if not chat:
        raise HTTPException(status_code=404, detail="Чат не найден")
    m = db.query(ChatMember).filter(ChatMember.chat_id == chat_id, ChatMember.user_id == user.id).first()
    if not m:
        raise HTTPException(status_code=403, detail="Нет доступа к чату")
    return chat


def _next_seq(db: Session) -> int:
    return db.execute(text("SELECT nextval('chat_message_seq')")).scalar()


# ── сериализация ──────────────────────────────────────────────────────────
def _snippet(body: str) -> str:
    s = body or ""
    s = re.sub(r"@\[audio\]\([^)]*\)", "🎤 Голосовое сообщение", s)
    s = re.sub(r"!\[[^\]]*\]\([^)]*\)", "🖼 Фото", s)
    s = re.sub(r"\s+", " ", s).strip()
    return s[:120]


def _msg_out(m: ChatMessage) -> ChatMessageOut:
    reply_preview = None
    if m.reply_to_id and getattr(m, "reply_to", None):
        reply_preview = _snippet(m.reply_to.body)
    return ChatMessageOut(
        id=m.id, chat_id=m.chat_id, seq=m.seq, client_uuid=m.client_uuid,
        author_id=m.author_id, author_name=m.author.full_name if m.author else None,
        body="" if m.deleted else m.body,
        reply_to_id=m.reply_to_id, reply_preview=reply_preview,
        created_at=m.created_at, edited_at=m.edited_at, edit_count=m.edit_count or 0, deleted=m.deleted,
    )


def _members_out(chat: Chat) -> list[ChatMemberOut]:
    out = []
    for mem in chat.members:
        out.append(ChatMemberOut(
            user_id=mem.user_id,
            full_name=mem.user.full_name if mem.user else None,
            avatar_url=mem.user.avatar_url if mem.user else None,
            role=mem.role, last_read_seq=mem.last_read_seq,
        ))
    return out


def _chat_out(db: Session, chat: Chat, user: User) -> ChatOut:
    last = (
        db.query(ChatMessage).filter(ChatMessage.chat_id == chat.id)
        .order_by(ChatMessage.seq.desc()).first()
    )
    me = next((m for m in chat.members if m.user_id == user.id), None)
    last_read = me.last_read_seq if me else 0
    unread = (
        db.query(func.count(ChatMessage.id))
        .filter(ChatMessage.chat_id == chat.id, ChatMessage.seq > last_read,
                ChatMessage.author_id != user.id, ChatMessage.deleted.is_(False))
        .scalar()
    ) or 0
    return ChatOut(
        id=chat.id, type=chat.type, title=chat.title, photo_url=chat.photo_url,
        created_by=chat.created_by, created_at=chat.created_at, updated_at=chat.updated_at,
        members=_members_out(chat), last_message=_msg_out(last) if last else None, unread=unread,
    )


async def _broadcast(db: Session, chat_id: int, data: dict) -> None:
    """Разослать событие участникам чата по их WS-сокетам."""
    try:
        from app.api.ws_chat import manager
        await manager.send_to_users(_member_ids(db, chat_id), data)
    except Exception:
        pass


# ── чаты ──────────────────────────────────────────────────────────────────
@router.get("", response_model=list[ChatOut])
def list_chats(db: Session = Depends(get_db), user: User = Depends(chat_user)):
    chats = (
        db.query(Chat)
        .options(joinedload(Chat.members).joinedload(ChatMember.user))
        .filter(Chat.id.in_(_my_chat_ids(db, user)))
        .order_by(Chat.updated_at.desc()).all()
    )
    return [_chat_out(db, c, user) for c in chats]


@router.get("/contacts", response_model=list[ContactOut])
def list_contacts(db: Session = Depends(get_db), user: User = Depends(chat_user)):
    """Пользователи, с которыми можно начать чат (активные, не ожидающие апрува, кроме себя)."""
    pending_disciples = select(Disciple.id).where(Disciple.is_approved.is_(False))
    users = (
        db.query(User)
        .filter(User.is_active.is_(True), User.id != user.id,
                or_(User.disciple_id.is_(None), User.disciple_id.notin_(pending_disciples)))
        .order_by(User.full_name).all()
    )
    return [ContactOut(id=u.id, full_name=u.full_name, avatar_url=u.avatar_url,
                       role=u.role.value if u.role else None) for u in users]


@router.get("/updates", response_model=UpdatesOut)
def get_updates(since: int = 0, limit: int = 300, db: Session = Depends(get_db), user: User = Depends(chat_user)):
    """Догон пропущенных сообщений: все сообщения в чатах пользователя с seq > since."""
    limit = max(1, min(limit, 500))
    rows = (
        db.query(ChatMessage)
        .options(joinedload(ChatMessage.author), joinedload(ChatMessage.reply_to))
        .filter(ChatMessage.chat_id.in_(_my_chat_ids(db, user)), ChatMessage.seq > since)
        .order_by(ChatMessage.seq.asc()).limit(limit + 1).all()
    )
    has_more = len(rows) > limit
    rows = rows[:limit]
    updates = [ChatUpdate(type="message", seq=m.seq, chat_id=m.chat_id, message=_msg_out(m)) for m in rows]
    pts = rows[-1].seq if rows else since
    return UpdatesOut(updates=updates, pts=pts, has_more=has_more)


@router.post("", response_model=ChatOut, status_code=status.HTTP_201_CREATED)
async def create_chat(payload: ChatCreateIn, db: Session = Depends(get_db), user: User = Depends(chat_user)):
    if payload.type == ChatType.direct:
        if not payload.peer_id or payload.peer_id == user.id:
            raise HTTPException(status_code=400, detail="Укажите собеседника")
        peer = db.get(User, payload.peer_id)
        if not peer or not peer.is_active or _is_pending(db, peer):
            raise HTTPException(status_code=404, detail="Собеседник недоступен")
        # дедуп: уже есть личный чат этих двоих?
        a = select(ChatMember.chat_id).where(ChatMember.user_id == user.id)
        b = select(ChatMember.chat_id).where(ChatMember.user_id == peer.id)
        existing = (
            db.query(Chat)
            .filter(Chat.type == ChatType.direct, Chat.id.in_(a), Chat.id.in_(b))
            .first()
        )
        if existing:
            db.refresh(existing)
            return _chat_out(db, existing, user)
        chat = Chat(type=ChatType.direct, created_by=user.id)
        db.add(chat)
        db.flush()
        db.add_all([
            ChatMember(chat_id=chat.id, user_id=user.id, role="member"),
            ChatMember(chat_id=chat.id, user_id=peer.id, role="member"),
        ])
        db.commit()
        db.refresh(chat)
        await _broadcast(db, chat.id, {"type": "chat", "chat_id": chat.id})
        return _chat_out(db, chat, user)

    # группа
    title = (payload.title or "").strip()
    if not title:
        raise HTTPException(status_code=400, detail="Укажите название группы")
    ids = set(payload.member_ids) - {user.id}
    valid = db.query(User).filter(User.id.in_(ids), User.is_active.is_(True)).all() if ids else []
    valid = [u for u in valid if not _is_pending(db, u)]
    chat = Chat(type=ChatType.group, title=title, created_by=user.id)
    db.add(chat)
    db.flush()
    members = [ChatMember(chat_id=chat.id, user_id=user.id, role="owner")]
    members += [ChatMember(chat_id=chat.id, user_id=u.id, role="member") for u in valid]
    db.add_all(members)
    db.commit()
    db.refresh(chat)
    await _broadcast(db, chat.id, {"type": "chat", "chat_id": chat.id})
    return _chat_out(db, chat, user)


@router.get("/{chat_id}", response_model=ChatOut)
def get_chat(chat_id: int, db: Session = Depends(get_db), user: User = Depends(chat_user)):
    chat = _require_membership(db, user, chat_id)
    db.refresh(chat)
    return _chat_out(db, chat, user)


@router.get("/{chat_id}/messages", response_model=list[ChatMessageOut])
def list_messages(chat_id: int, before_seq: int | None = None, limit: int = 50,
                  db: Session = Depends(get_db), user: User = Depends(chat_user)):
    _require_membership(db, user, chat_id)
    limit = max(1, min(limit, 100))
    q = (
        db.query(ChatMessage)
        .options(joinedload(ChatMessage.author), joinedload(ChatMessage.reply_to))
        .filter(ChatMessage.chat_id == chat_id)
    )
    if before_seq:
        q = q.filter(ChatMessage.seq < before_seq)
    rows = q.order_by(ChatMessage.seq.desc()).limit(limit).all()
    rows.reverse()  # по возрастанию для отображения
    return [_msg_out(m) for m in rows]


@router.post("/{chat_id}/messages", response_model=ChatMessageOut, status_code=status.HTTP_201_CREATED)
async def send_message(chat_id: int, payload: SendMessageIn, db: Session = Depends(get_db), user: User = Depends(chat_user)):
    chat = _require_membership(db, user, chat_id)
    body = (payload.body or "").strip()
    if not body:
        raise HTTPException(status_code=400, detail="Пустое сообщение")
    if not payload.client_uuid:
        raise HTTPException(status_code=400, detail="Нужен client_uuid")

    # идемпотентность: повтор с тем же uuid не создаёт дубль
    existing = (
        db.query(ChatMessage)
        .options(joinedload(ChatMessage.author), joinedload(ChatMessage.reply_to))
        .filter(ChatMessage.chat_id == chat_id, ChatMessage.client_uuid == payload.client_uuid).first()
    )
    if existing:
        return _msg_out(existing)

    reply_to_id = None
    if payload.reply_to_id:
        parent = db.get(ChatMessage, payload.reply_to_id)
        if parent and parent.chat_id == chat_id:
            reply_to_id = parent.id

    msg = ChatMessage(
        chat_id=chat_id, seq=_next_seq(db), client_uuid=payload.client_uuid,
        author_id=user.id, body=body, reply_to_id=reply_to_id,
    )
    db.add(msg)
    chat.updated_at = func.now()
    # автор сразу «прочитал» до своего сообщения
    me = db.query(ChatMember).filter(ChatMember.chat_id == chat_id, ChatMember.user_id == user.id).first()
    try:
        db.flush()
        if me and msg.seq > me.last_read_seq:
            me.last_read_seq = msg.seq
        db.commit()
    except IntegrityError:
        db.rollback()  # гонка одинаковых uuid — вернуть уже созданное
        existing = (
            db.query(ChatMessage)
            .options(joinedload(ChatMessage.author), joinedload(ChatMessage.reply_to))
            .filter(ChatMessage.chat_id == chat_id, ChatMessage.client_uuid == payload.client_uuid).first()
        )
        if existing:
            return _msg_out(existing)
        raise
    db.refresh(msg)
    out = _msg_out(msg)
    await _broadcast(db, chat_id, {"type": "message", "seq": msg.seq, "chat_id": chat_id, "message": out.model_dump(mode="json")})
    return out


@router.patch("/{chat_id}/messages/{message_id}", response_model=ChatMessageOut)
async def edit_message(chat_id: int, message_id: int, payload: EditMessageIn,
                       db: Session = Depends(get_db), user: User = Depends(chat_user)):
    _require_membership(db, user, chat_id)
    msg = db.get(ChatMessage, message_id)
    if not msg or msg.chat_id != chat_id or msg.deleted:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    if msg.author_id != user.id:
        raise HTTPException(status_code=403, detail="Можно менять только свои сообщения")
    created = msg.created_at
    if created and created.tzinfo is None:
        created = created.replace(tzinfo=timezone.utc)
    if created and (datetime.now(timezone.utc) - created) > EDIT_WINDOW:
        raise HTTPException(status_code=403, detail="Срок редактирования истёк")
    body = (payload.body or "").strip()
    if not body:
        raise HTTPException(status_code=400, detail="Пустое сообщение")
    msg.body = body
    msg.edited_at = func.now()
    msg.edit_count = (msg.edit_count or 0) + 1
    db.commit()
    db.refresh(msg)
    out = _msg_out(msg)
    await _broadcast(db, chat_id, {"type": "edit", "chat_id": chat_id, "message": out.model_dump(mode="json")})
    return out


@router.delete("/{chat_id}/messages/{message_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_message(chat_id: int, message_id: int, db: Session = Depends(get_db), user: User = Depends(chat_user)):
    _require_membership(db, user, chat_id)
    msg = db.get(ChatMessage, message_id)
    if not msg or msg.chat_id != chat_id:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    if msg.author_id != user.id:
        raise HTTPException(status_code=403, detail="Можно удалять только свои сообщения")
    msg.deleted = True
    msg.body = ""
    db.commit()
    await _broadcast(db, chat_id, {"type": "delete", "chat_id": chat_id, "message_id": message_id})


@router.post("/{chat_id}/read", status_code=status.HTTP_204_NO_CONTENT)
async def mark_read(chat_id: int, payload: ReadIn, db: Session = Depends(get_db), user: User = Depends(chat_user)):
    _require_membership(db, user, chat_id)
    me = db.query(ChatMember).filter(ChatMember.chat_id == chat_id, ChatMember.user_id == user.id).first()
    if me and payload.seq > me.last_read_seq:
        me.last_read_seq = payload.seq
        db.commit()
        await _broadcast(db, chat_id, {"type": "read", "chat_id": chat_id, "user_id": user.id, "last_read_seq": payload.seq})
