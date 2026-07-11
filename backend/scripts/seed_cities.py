"""Наполнить справочник городов уникальными городами из карточек учеников.

    python -m scripts.seed_cities
Идемпотентно: существующие города не дублируются (сравнение без учёта регистра).
"""
from app.core.database import SessionLocal
from app.models import City, Disciple


def main():
    db = SessionLocal()
    added = 0
    try:
        existing = {c.name.lower() for c in db.query(City).all()}
        rows = db.query(Disciple.city, Disciple.country).filter(Disciple.city.isnot(None)).all()
        seen = set()
        for city, country in rows:
            name = (city or "").strip()
            if not name or name.lower() in existing or name.lower() in seen:
                continue
            seen.add(name.lower())
            db.add(City(name=name, country=country))
            added += 1
        db.commit()
        print(f"[seed_cities] добавлено: {added}")
    finally:
        db.close()


if __name__ == "__main__":
    main()
