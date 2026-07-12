from datetime import datetime

from sqlalchemy import DateTime, Enum, ForeignKey, String, UniqueConstraint, func
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base
from app.core.enums import ThreadKind


class Thread(Base):
    """Ветка общения по ученику: вопрос гуру или отчёт о служении."""

    __tablename__ = "threads"

    id: Mapped[int] = mapped_column(primary_key=True)
    kind: Mapped[ThreadKind] = mapped_column(Enum(ThreadKind, native_enum=False), nullable=False, index=True)
    disciple_id: Mapped[int] = mapped_column(ForeignKey("disciples.id", ondelete="CASCADE"), nullable=False, index=True)
    subject: Mapped[str | None] = mapped_column(String(255), nullable=True)
    period: Mapped[str | None] = mapped_column(String(7), nullable=True)  # 'YYYY-MM' для отчётов

    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())
    # когда сторона-получатель (с правами) последний раз смотрела ветку — общий счётчик «непросмотренных»
    staff_seen_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)

    disciple = relationship("Disciple")
    messages = relationship(
        "ThreadMessage", back_populates="thread", cascade="all, delete-orphan", order_by="ThreadMessage.created_at"
    )


class ThreadMessage(Base):
    __tablename__ = "thread_messages"

    id: Mapped[int] = mapped_column(primary_key=True)
    thread_id: Mapped[int] = mapped_column(ForeignKey("threads.id", ondelete="CASCADE"), nullable=False, index=True)
    author_id: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True)
    body: Mapped[str] = mapped_column(String, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    # редактирование в течение часа: отметка и число правок
    edited_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    edit_count: Mapped[int] = mapped_column(default=0, server_default="0", nullable=False)
    # ответ на сообщение (цитирование)
    reply_to_id: Mapped[int | None] = mapped_column(ForeignKey("thread_messages.id", ondelete="SET NULL"), nullable=True)

    thread = relationship("Thread", back_populates="messages")
    author = relationship("User")
    reply_to = relationship("ThreadMessage", remote_side=[id], foreign_keys=[reply_to_id], uselist=False)
    likes = relationship("MessageLike", back_populates="message", cascade="all, delete-orphan")


class ThreadRead(Base):
    """Отметка: когда пользователь в последний раз открывал ветку (для индикатора «новое»)."""

    __tablename__ = "thread_reads"
    __table_args__ = (UniqueConstraint("thread_id", "user_id", name="uq_thread_read"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    thread_id: Mapped[int] = mapped_column(ForeignKey("threads.id", ondelete="CASCADE"), nullable=False, index=True)
    user_id: Mapped[int] = mapped_column(ForeignKey("users.id", ondelete="CASCADE"), nullable=False, index=True)
    last_seen_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())


class MessageLike(Base):
    """Лайк на сообщение (отчёт ученика) от гуру или наставника."""

    __tablename__ = "message_likes"
    __table_args__ = (UniqueConstraint("message_id", "user_id", name="uq_message_like"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    message_id: Mapped[int] = mapped_column(ForeignKey("thread_messages.id", ondelete="CASCADE"), nullable=False, index=True)
    user_id: Mapped[int] = mapped_column(ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    emoji: Mapped[str] = mapped_column(String(16), default="❤️", server_default="❤️", nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())

    message = relationship("ThreadMessage", back_populates="likes")
