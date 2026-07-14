"""Создать/обновить супер-администратора (отдельный вход, не под гуру).

Usage:
    python -m app.create_admin <email> <password> [ФИО]

Идемпотентно: заводит пользователя, назначает системную роль superadmin
(она даёт все права), а также legacy-роль guru — чтобы проходили старые
проверки (staff/guru), запись голосовых и т.п.
"""
import sys

from app.core.database import SessionLocal
from app.core.enums import Role as LegacyRole
from app.core.capabilities import seed_roles
from app.core.security import hash_password
from app.models import Role, User, UserRole


def main():
    if len(sys.argv) < 3:
        print("usage: python -m app.create_admin <email> <password> [ФИО]")
        sys.exit(1)
    email = sys.argv[1].strip().lower()
    password = sys.argv[2]
    name = sys.argv[3] if len(sys.argv) > 3 else "Супер-администратор"

    db = SessionLocal()
    try:
        seed_roles(db)  # гарантируем наличие роли superadmin
        sa = db.query(Role).filter(Role.key == "superadmin").first()
        if not sa:
            print("[!] системная роль superadmin не найдена")
            sys.exit(2)

        u = db.query(User).filter(User.email == email).first()
        if u:
            u.hashed_password = hash_password(password)
            u.is_active = True
            u.role = LegacyRole.guru
            print(f"[=] обновлён: {email}")
        else:
            u = User(email=email, full_name=name, role=LegacyRole.guru,
                     is_active=True, hashed_password=hash_password(password))
            db.add(u)
            print(f"[+] создан: {email}")
        db.commit()
        db.refresh(u)

        if not db.query(UserRole).filter(UserRole.user_id == u.id, UserRole.role_id == sa.id).first():
            db.add(UserRole(user_id=u.id, role_id=sa.id))
            db.commit()
            print("[+] назначена роль superadmin")
        else:
            print("[=] роль superadmin уже назначена")
    finally:
        db.close()


if __name__ == "__main__":
    main()
