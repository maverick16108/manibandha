from datetime import datetime

from sqlalchemy import DateTime, Enum, ForeignKey, String, func
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

    thread = relationship("Thread", back_populates="messages")
    author = relationship("User")
