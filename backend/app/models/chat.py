"""Local-first мессенджер: чаты (личные и групповые), участники, сообщения.

Синхронизация — в стиле Telegram (сервер авторитетен, без CRDT):
- у каждого сообщения глобальный монотонный `seq` (из sequence `chat_message_seq`);
- клиент шлёт `client_uuid` для идемпотентной отправки (повтор не создаёт дубль);
- догон пропущенного — через GET /chats/updates?since={pts}, где pts = максимальный
  применённый `seq`.
"""
from datetime import datetime

from sqlalchemy import (
    BigInteger, Boolean, DateTime, Enum, ForeignKey, Integer, String, Text, UniqueConstraint, func,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base
from app.core.enums import ChatType

# Глобальная монотонная последовательность порядка сообщений (создаётся в миграции).
CHAT_MESSAGE_SEQ = "chat_message_seq"


class Chat(Base):
    __tablename__ = "chats"

    id: Mapped[int] = mapped_column(primary_key=True)
    type: Mapped[ChatType] = mapped_column(Enum(ChatType, native_enum=False), nullable=False, index=True)
    title: Mapped[str | None] = mapped_column(String(255), nullable=True)  # название группы
    photo_url: Mapped[str | None] = mapped_column(String(512), nullable=True)  # аватар группы
    created_by: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())

    members = relationship("ChatMember", back_populates="chat", cascade="all, delete-orphan")
    messages = relationship("ChatMessage", back_populates="chat", cascade="all, delete-orphan")


class ChatMember(Base):
    __tablename__ = "chat_members"
    __table_args__ = (UniqueConstraint("chat_id", "user_id", name="uq_chat_member"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    chat_id: Mapped[int] = mapped_column(ForeignKey("chats.id", ondelete="CASCADE"), nullable=False, index=True)
    user_id: Mapped[int] = mapped_column(ForeignKey("users.id", ondelete="CASCADE"), nullable=False, index=True)
    role: Mapped[str] = mapped_column(String(16), default="member", server_default="member", nullable=False)  # owner | member
    # до какого seq пользователь прочитал чат (для галочек/непрочитанного)
    last_read_seq: Mapped[int] = mapped_column(BigInteger, default=0, server_default="0", nullable=False)
    joined_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())

    chat = relationship("Chat", back_populates="members")
    user = relationship("User")


class ChatMessage(Base):
    __tablename__ = "chat_messages"
    __table_args__ = (
        UniqueConstraint("chat_id", "client_uuid", name="uq_chat_message_uuid"),  # идемпотентность отправки
    )

    id: Mapped[int] = mapped_column(primary_key=True)
    chat_id: Mapped[int] = mapped_column(ForeignKey("chats.id", ondelete="CASCADE"), nullable=False, index=True)
    # глобальный порядок доставки; назначается из sequence при вставке
    seq: Mapped[int] = mapped_column(BigInteger, nullable=False, index=True, unique=True)
    client_uuid: Mapped[str | None] = mapped_column(String(64), nullable=True)
    author_id: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True)
    body: Mapped[str] = mapped_column(Text, nullable=False)  # текст/markdown; голосовое — токен @[audio](url)
    reply_to_id: Mapped[int | None] = mapped_column(ForeignKey("chat_messages.id", ondelete="SET NULL"), nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    edited_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    edit_count: Mapped[int] = mapped_column(Integer, default=0, server_default="0", nullable=False)
    deleted: Mapped[bool] = mapped_column(Boolean, default=False, server_default="false", nullable=False)

    chat = relationship("Chat", back_populates="messages")
    author = relationship("User")
    reply_to = relationship("ChatMessage", remote_side=[id], foreign_keys=[reply_to_id], uselist=False)
    reactions = relationship("ChatMessageReaction", back_populates="message", cascade="all, delete-orphan")


class ChatMessageReaction(Base):
    """Реакция-эмодзи на сообщение чата (одна на пользователя на сообщение)."""

    __tablename__ = "chat_message_reactions"
    __table_args__ = (UniqueConstraint("message_id", "user_id", name="uq_chat_reaction"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    message_id: Mapped[int] = mapped_column(ForeignKey("chat_messages.id", ondelete="CASCADE"), nullable=False, index=True)
    user_id: Mapped[int] = mapped_column(ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    emoji: Mapped[str] = mapped_column(String(16), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())

    message = relationship("ChatMessage", back_populates="reactions")
