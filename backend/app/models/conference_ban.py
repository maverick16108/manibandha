from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, String, UniqueConstraint, func
from sqlalchemy.orm import Mapped, mapped_column

from app.core.database import Base


class ConferenceBan(Base):
    """Участник, удалённый ведущим из конференции (не может зайти, пока в списке)."""

    __tablename__ = "conference_bans"
    __table_args__ = (UniqueConstraint("conference_id", "identity", name="uq_conf_ban"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    conference_id: Mapped[int] = mapped_column(ForeignKey("conferences.id", ondelete="CASCADE"), nullable=False, index=True)
    identity: Mapped[str] = mapped_column(String(80), nullable=False)
    name: Mapped[str | None] = mapped_column(String(120), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
