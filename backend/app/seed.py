"""Idempotent seed: creates the first guru account, and (optionally) demo data.

Usage:
    python -m app.seed          # first guru only
    python -m app.seed --demo   # + sample temples and disciples
"""
import sys
from datetime import date

from app.core.config import settings
from app.core.database import SessionLocal
from app.core.enums import InitiationStatus, MaritalStatus, Role
from app.core.security import hash_password
from app.models import ChecklistItem, Disciple, Temple, User


def ensure_first_guru(db) -> User:
    guru = db.query(User).filter(User.email == settings.FIRST_GURU_EMAIL).first()
    if guru:
        print(f"[=] guru already exists: {guru.email}")
        return guru
    guru = User(
        email=settings.FIRST_GURU_EMAIL,
        full_name=settings.FIRST_GURU_NAME,
        role=Role.guru,
        is_active=True,
        hashed_password=hash_password(settings.FIRST_GURU_PASSWORD),
    )
    db.add(guru)
    db.commit()
    db.refresh(guru)
    print(f"[+] created guru: {guru.email}")
    return guru


def seed_demo(db, guru: User):
    if db.query(Disciple).count() > 0:
        print("[=] demo data skipped (disciples already exist)")
        return

    curator = db.query(User).filter(User.email == "curator@manibandha.local").first()
    if not curator:
        curator = User(
            email="curator@manibandha.local",
            full_name="Наставник прабху",
            role=Role.curator,
            hashed_password=hash_password("change-me"),
        )
        db.add(curator)
        db.commit()
        db.refresh(curator)

    mayapur = Temple(name="Шри Маяпур", city="Маяпур", country="Индия", president_name="Джаяпатака Свами")
    moscow = Temple(name="Москва центр", city="Москва", country="Россия", president_name="Президент храма")
    db.add_all([mayapur, moscow])
    db.commit()

    d1 = Disciple(
        material_name="Иван Петров", spiritual_name=None, country="Россия", city="Москва",
        temple_id=moscow.id, marital_status=MaritalStatus.single, date_of_birth=date(1990, 5, 12),
        initiation_status=InitiationStatus.aspirant, mentor_id=curator.id,
        recommended_by="Президент храма Москва", application_date=date(2025, 1, 10),
        seva="Кухня (прасадам)", current_activity="Учёба",
    )
    d2 = Disciple(
        material_name="Алексей Смирнов", spiritual_name="Ананта дас", country="Россия", city="Санкт-Петербург",
        temple_id=moscow.id, marital_status=MaritalStatus.married,
        initiation_status=InitiationStatus.harinama, harinama_date=date(2022, 8, 20),
        harinama_name="Ананта дас", mentor_id=curator.id, ready_for_initiation=True,
        recommended_by="Наставник прабху", seva="Проповедь", current_activity="Работа + проповедь",
    )
    d3 = Disciple(
        material_name="Мария Иванова", spiritual_name="Радха деви даси", country="Украина", city="Киев",
        marital_status=MaritalStatus.single, initiation_status=InitiationStatus.brahman,
        harinama_date=date(2018, 3, 15), harinama_name="Радха деви даси", brahman_date=date(2021, 4, 1),
        mentor_id=curator.id, seva="Пуджа", current_activity="Служение в храме",
    )
    db.add_all([d1, d2, d3])
    db.commit()

    for title in ("Стаж в сознании Кришны не менее 1 года", "Следование обетам", "Отсутствие возражений от общины"):
        db.add(ChecklistItem(disciple_id=d1.id, title=title, is_done=False, target=InitiationStatus.harinama))
    db.commit()
    print("[+] demo data created")


def main():
    db = SessionLocal()
    try:
        guru = ensure_first_guru(db)
        if "--demo" in sys.argv:
            seed_demo(db, guru)
    finally:
        db.close()


if __name__ == "__main__":
    main()
