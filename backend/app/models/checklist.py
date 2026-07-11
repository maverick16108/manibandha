from datetime import datetime

from sqlalchemy import Boolean, DateTime, Enum, ForeignKey, String, Text, func
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base
from app.core.enums import InitiationStatus


class ChecklistItem(Base):
    """A requirement on the aspirant path before an initiation
    (стаж, обеты, отсутствие возражений, ...)."""

    __tablename__ = "checklist_items"

    id: Mapped[int] = mapped_column(primary_key=True)
    disciple_id: Mapped[int] = mapped_column(ForeignKey("disciples.id", ondelete="CASCADE"), nullable=False, index=True)
    title: Mapped[str] = mapped_column(String(512), nullable=False)
    is_done: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)
    note: Mapped[str | None] = mapped_column(Text, nullable=True)
    # Which initiation this requirement gates.
    target: Mapped[InitiationStatus] = mapped_column(
        Enum(InitiationStatus, native_enum=False), default=InitiationStatus.harinama, nullable=False
    )
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())

    disciple = relationship("Disciple", back_populates="checklist")
