from datetime import date, datetime

from sqlalchemy import Boolean, Date, DateTime, Enum, ForeignKey, String, Text, func
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base
from app.core.enums import InitiationStatus, MaritalStatus


class Disciple(Base):
    __tablename__ = "disciples"

    id: Mapped[int] = mapped_column(primary_key=True)

    # Names
    spiritual_name: Mapped[str | None] = mapped_column(String(255), nullable=True, index=True)
    material_name: Mapped[str] = mapped_column(String(255), nullable=False, index=True)
    photo_url: Mapped[str | None] = mapped_column(String(512), nullable=True)

    # Contacts
    phone: Mapped[str | None] = mapped_column(String(64), nullable=True)
    email: Mapped[str | None] = mapped_column(String(255), nullable=True)
    messenger: Mapped[str | None] = mapped_column(String(255), nullable=True)

    # Location / community
    country: Mapped[str | None] = mapped_column(String(120), nullable=True, index=True)
    city: Mapped[str | None] = mapped_column(String(120), nullable=True)
    temple_id: Mapped[int | None] = mapped_column(ForeignKey("temples.id", ondelete="SET NULL"), nullable=True)

    # Personal
    marital_status: Mapped[MaritalStatus | None] = mapped_column(Enum(MaritalStatus, native_enum=False), nullable=True)
    date_of_birth: Mapped[date | None] = mapped_column(Date, nullable=True)

    # Initiation
    initiation_status: Mapped[InitiationStatus] = mapped_column(
        Enum(InitiationStatus, native_enum=False), nullable=False, default=InitiationStatus.aspirant, index=True
    )
    harinama_date: Mapped[date | None] = mapped_column(Date, nullable=True)
    harinama_name: Mapped[str | None] = mapped_column(String(255), nullable=True)  # духовное имя на харинаме
    brahman_date: Mapped[date | None] = mapped_column(Date, nullable=True)

    # Service / activity
    seva: Mapped[str | None] = mapped_column(Text, nullable=True)
    current_activity: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Pipeline (aspirant path)
    mentor_id: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True, index=True)
    recommended_by: Mapped[str | None] = mapped_column(String(255), nullable=True)  # наставник / президент храма
    application_date: Mapped[date | None] = mapped_column(Date, nullable=True)
    ready_for_initiation: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)

    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())

    temple = relationship("Temple", back_populates="disciples")
    mentor = relationship("User", back_populates="mentored", foreign_keys=[mentor_id])
    checklist = relationship("ChecklistItem", back_populates="disciple", cascade="all, delete-orphan")
