"""Каталог прав-действий (capabilities) и системные роли.

Права сгруппированы по фичам. Роль = набор прав. У пользователя может быть
несколько ролей; его права — объединение прав всех ролей. Роль-superadmin (гуру)
всегда имеет все права, включая будущие.
"""
from sqlalchemy.orm import Session

# (key, label, group) — каталог прав. Порядок влияет на отображение.
CAPABILITIES = [
    ("dashboard.view", "Смотреть обзор", "Обзор"),
    ("calendar.view", "Смотреть календарь", "Календарь"),
    ("calendar.manage", "Управлять событиями", "Календарь"),
    ("disciples.view_all", "Видеть всех учеников", "Ученики"),
    ("disciples.view_own", "Видеть своих учеников", "Ученики"),
    ("disciples.create", "Добавлять учеников", "Ученики"),
    ("disciples.edit", "Редактировать анкеты", "Ученики"),
    ("disciples.delete", "Удалять учеников", "Ученики"),
    ("disciples.approve", "Апрувить регистрации", "Ученики"),
    ("disciples.note", "Делать заметки об учениках", "Ученики"),
    ("questions.ask", "Задавать вопросы", "Вопросы"),
    ("questions.answer", "Отвечать на вопросы", "Вопросы"),
    ("questions.view_all", "Видеть все вопросы", "Вопросы"),
    ("reports.write", "Писать отчёты", "Отчёты о служении"),
    ("reports.read_all", "Читать отчёты учеников", "Отчёты о служении"),
    ("reports.like", "Ставить лайки отчётам", "Отчёты о служении"),
    ("forum.view", "Читать форум", "Форум"),
    ("forum.post", "Писать на форуме", "Форум"),
    ("forum.moderate", "Модерировать форум", "Форум"),
    ("conference.view", "Участвовать в конференциях", "Конференция"),
    ("conference.host", "Проводить конференции", "Конференция"),
    ("dictionaries.manage", "Управлять справочниками", "Справочники"),
    ("users.manage", "Управлять пользователями", "Пользователи"),
    ("roles.manage", "Управлять ролями", "Роли"),
    ("settings.manage", "Управлять настройками", "Настройки"),
]
ALL_CAPS = [k for k, _, _ in CAPABILITIES]
CAP_KEYS = set(ALL_CAPS)

# Предопределённые (системные) роли. Гуру — superadmin (все права).
SYSTEM_ROLES = [
    {"key": "superadmin", "name": "Супер-администратор", "is_superadmin": True, "is_default": False, "capabilities": []},
    {"key": "guru", "name": "Гуру", "is_superadmin": True, "is_default": False, "capabilities": []},
    {"key": "secretary", "name": "Секретарь", "is_superadmin": False, "is_default": False, "capabilities": [
        "dashboard.view", "calendar.view", "calendar.manage",
        "disciples.view_all", "disciples.create", "disciples.edit", "disciples.delete", "disciples.approve",
        "disciples.note", "forum.view", "forum.post", "forum.moderate",
        "conference.view", "conference.host",
        "dictionaries.manage", "users.manage",
    ]},
    {"key": "curator", "name": "Куратор", "is_superadmin": False, "is_default": False, "capabilities": [
        "dashboard.view", "calendar.view", "disciples.view_own", "disciples.edit",
        "disciples.note", "reports.read_all", "reports.like", "forum.view", "forum.post",
        "conference.view",
    ]},
    {"key": "student", "name": "Ученик", "is_superadmin": False, "is_default": True, "capabilities": [
        "calendar.view", "questions.ask", "reports.write", "forum.view", "forum.post",
        "conference.view",
    ]},
    {"key": "forum_moderator", "name": "Модератор форума", "is_superadmin": False, "is_default": False, "capabilities": [
        "forum.view", "forum.post", "forum.moderate",
    ]},
]


def capabilities_grouped():
    """Каталог прав для редактора ролей, сгруппированный по фичам."""
    groups = {}
    for key, label, group in CAPABILITIES:
        groups.setdefault(group, []).append({"key": key, "label": label})
    return [{"group": g, "items": items} for g, items in groups.items()]


def seed_roles(db: Session):
    """Создать недостающие системные роли и добавить новые базовые права (идемпотентно)."""
    from app.models import Role
    for spec in SYSTEM_ROLES:
        role = db.query(Role).filter(Role.key == spec["key"]).first()
        if not role:
            db.add(Role(
                key=spec["key"], name=spec["name"], is_system=True,
                is_superadmin=spec["is_superadmin"], is_default=spec["is_default"],
                capabilities=list(spec["capabilities"]),
            ))
        else:
            # добить недостающие базовые права системной роли (напр. новые фичи), не удаляя ручные
            missing = [c for c in spec["capabilities"] if c not in (role.capabilities or [])]
            if missing:
                role.capabilities = list(role.capabilities or []) + missing
    db.commit()


def user_roles(db: Session, user) -> list:
    from app.models import Role, UserRole
    return (
        db.query(Role)
        .join(UserRole, UserRole.role_id == Role.id)
        .filter(UserRole.user_id == user.id)
        .all()
    )


def user_capabilities(db: Session, user) -> set:
    """Объединение прав всех ролей пользователя (superadmin → все)."""
    roles = user_roles(db, user)
    if any(r.is_superadmin for r in roles):
        return set(ALL_CAPS)
    caps = set()
    for r in roles:
        caps.update(r.capabilities or [])
    return caps & CAP_KEYS


def user_is_superadmin(db: Session, user) -> bool:
    return any(r.is_superadmin for r in user_roles(db, user))


def has_cap(db: Session, user, cap: str) -> bool:
    return cap in user_capabilities(db, user)
