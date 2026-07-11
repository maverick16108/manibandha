from sqlalchemy import Boolean, Enum, String, UniqueConstraint
from sqlalchemy.orm import Mapped, mapped_column

from app.core.database import Base
from app.core.enums import Role


class RolePermission(Base):
    """Доступ роли к разделу системы (настраивается гуру)."""

    __tablename__ = "role_permissions"
    __table_args__ = (UniqueConstraint("role", "section", name="uq_role_section"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    role: Mapped[Role] = mapped_column(Enum(Role, native_enum=False), nullable=False, index=True)
    section: Mapped[str] = mapped_column(String(40), nullable=False)
    allowed: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)
