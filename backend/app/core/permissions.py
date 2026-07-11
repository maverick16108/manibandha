"""Права ролей на разделы. Гуру всегда имеет полный доступ (не блокируется)."""
from sqlalchemy.orm import Session

from app.core.enums import Role
from app.models import RolePermission

# ключи разделов = имена маршрутов на фронте
SECTIONS = [
    ("dashboard", "Обзор"),
    ("calendar", "Календарь"),
    ("disciples", "Ученики"),
    ("questions", "Вопросы"),
    ("service-reports", "Отчёты о служении"),
    ("dictionaries", "Справочники"),
    ("users", "Пользователи"),
]
SECTION_KEYS = [k for k, _ in SECTIONS]

# доступ по умолчанию (гуру опущен — у него всегда всё)
DEFAULTS = {
    Role.secretary: {"dashboard", "calendar", "disciples", "dictionaries", "users"},
    Role.curator: {"dashboard", "calendar", "disciples", "service-reports"},
    Role.student: {"calendar", "questions", "service-reports"},
}


def role_sections(db: Session, role: Role) -> dict[str, bool]:
    """Карта раздел->доступ для роли. Гуру — всё True."""
    if role == Role.guru:
        return {k: True for k in SECTION_KEYS}
    rows = {p.section: p.allowed for p in db.query(RolePermission).filter(RolePermission.role == role).all()}
    if not rows:  # ещё не настроено — берём дефолты
        return {k: (k in DEFAULTS.get(role, set())) for k in SECTION_KEYS}
    return {k: rows.get(k, False) for k in SECTION_KEYS}


def full_matrix(db: Session) -> dict[str, dict[str, bool]]:
    return {r.value: role_sections(db, r) for r in Role}


def is_allowed(db: Session, role: Role, section: str) -> bool:
    return role == Role.guru or role_sections(db, role).get(section, False)


def save_matrix(db: Session, matrix: dict[str, dict[str, bool]]):
    for role_value, sections in matrix.items():
        try:
            role = Role(role_value)
        except ValueError:
            continue
        if role == Role.guru:
            continue  # гуру не редактируем
        for section, allowed in sections.items():
            if section not in SECTION_KEYS:
                continue
            row = db.query(RolePermission).filter(
                RolePermission.role == role, RolePermission.section == section
            ).first()
            if row:
                row.allowed = bool(allowed)
            else:
                db.add(RolePermission(role=role, section=section, allowed=bool(allowed)))
    db.commit()
