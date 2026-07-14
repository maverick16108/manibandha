from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Integer, String, Text, func
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class DiscipleNote(Base):
    """Заметка куратора об ученике: дата + текст + автор."""

    __tablename__ = "disciple_notes"

    id: Mapped[int] = mapped_column(primary_key=True)
    disciple_id: Mapped[int] = mapped_column(ForeignKey("disciples.id", ondelete="CASCADE"), nullable=False, index=True)
    author_id: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True)
    text: Mapped[str] = mapped_column(Text, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())

    author = relationship("User")


class DiscipleFile(Base):
    """Файл, прикреплённый к анкете ученика."""

    __tablename__ = "disciple_files"

    id: Mapped[int] = mapped_column(primary_key=True)
    disciple_id: Mapped[int] = mapped_column(ForeignKey("disciples.id", ondelete="CASCADE"), nullable=False, index=True)
    uploaded_by: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True)
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    url: Mapped[str] = mapped_column(String(500), nullable=False)
    size: Mapped[int | None] = mapped_column(Integer, nullable=True)
    content_type: Mapped[str | None] = mapped_column(String(120), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())

    uploader = relationship("User")
