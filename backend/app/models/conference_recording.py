from datetime import datetime

from sqlalchemy import BigInteger, DateTime, ForeignKey, Integer, String, func
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class ConferenceRecording(Base):
    """Запись конференции (LiveKit Egress → MP4 в архиве на сервере)."""

    __tablename__ = "conference_recordings"

    id: Mapped[int] = mapped_column(primary_key=True)
    conference_id: Mapped[int] = mapped_column(ForeignKey("conferences.id", ondelete="CASCADE"), nullable=False, index=True)
    egress_id: Mapped[str | None] = mapped_column(String(80), nullable=True, index=True)
    filename: Mapped[str | None] = mapped_column(String(255), nullable=True)  # имя файла в каталоге записей
    status: Mapped[str] = mapped_column(String(20), default="active", server_default="active", nullable=False)  # active | done | failed
    duration_ms: Mapped[int] = mapped_column(BigInteger, default=0, server_default="0", nullable=False)
    size_bytes: Mapped[int] = mapped_column(BigInteger, default=0, server_default="0", nullable=False)
    started_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    ended_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)

    conference = relationship("Conference")
