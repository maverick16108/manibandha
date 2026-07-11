"""Нормализовать города (с заглавной буквы) и проставить область каждому ученику по городу.

    python -m scripts.normalize_cities
"""
from app.core.database import SessionLocal
from app.models import City, Disciple

# Канонический город -> регион (субъект РФ). Названия регионов совпадают со справочником.
HMAO = "Ханты-Мансийский автономный округ — Югра"
CITY_REGION = {
    "Иркутск": "Иркутская область",
    "Омск": "Омская область",
    "Новый Уренгой": "Ямало-Ненецкий автономный округ",
    "Москва": "Москва",
    "Кунгур": "Пермский край",
    "Бузулук": "Оренбургская область",
    "Дегтярск": "Свердловская область",
    "Кумертау": "Республика Башкортостан",
    "Кирово-Чепецк": "Кировская область",
    "Киров": "Кировская область",
    "Тюмень": "Тюменская область",
    "Екатеринбург": "Свердловская область",
    "Челябинск": "Челябинская область",
    "Чайковский": "Пермский край",
    "Уфа": "Республика Башкортостан",
    "Сибай": "Республика Башкортостан",
    "Миасс": "Челябинская область",
    "Сургут": HMAO,
    "Нижний Тагил": "Свердловская область",
    "Липецк": "Липецкая область",
    "Красноярск": "Красноярский край",
    "Ижевск": "Удмуртская Республика",
    "Нижневартовск": HMAO,
    "Ханты-Мансийск": HMAO,
    "Можга": "Удмуртская Республика",
    "Октябрьский": "Республика Башкортостан",
    "Радужный": HMAO,
    "Тамбов": "Тамбовская область",
    "Учалы": "Республика Башкортостан",
    "Санкт-Петербург": "Санкт-Петербург",
}
# сортируем по длине, чтобы «Кирово-Чепецк» матчился раньше «Киров»
CANON = sorted(CITY_REGION.keys(), key=len, reverse=True)

# частые опечатки/варианты написания -> канонический город
ALIASES = {"нидневартовск": "Нижневартовск"}


def cap(s: str) -> str:
    s = s.strip()
    return s[0].upper() + s[1:] if s else s


def canonical(raw: str):
    """('Сургут-югра' | 'Можга, Удмуртия') -> ('Сургут', region) | (cap(raw), None)."""
    low = raw.strip().lower()
    for alias, city in ALIASES.items():
        if low.startswith(alias):
            return city, CITY_REGION[city]
    for city in CANON:
        if low.startswith(city.lower()):
            return city, CITY_REGION[city]
    return cap(raw), None


def main():
    db = SessionLocal()
    changed = regioned = 0
    try:
        for d in db.query(Disciple).filter(Disciple.city.isnot(None)).all():
            city, region = canonical(d.city)
            if city != d.city:
                d.city = city
                changed += 1
            if region and d.region != region:
                d.region = region
                regioned += 1
        db.commit()

        # Справочник городов: нормализовать имена и убрать дубли.
        # Группируем по нормализованному имени, оставляем одного, остальных удаляем.
        groups = {}
        for c in db.query(City).order_by(City.id).all():
            groups.setdefault(canonical(c.name)[0], []).append(c)
        # сначала удаляем дубликаты и сбрасываем в БД, чтобы освободить уникальные имена
        survivors = {}
        for name, recs in groups.items():
            survivors[name] = recs[0]
            for dup in recs[1:]:
                db.delete(dup)
        db.flush()
        # затем переименовываем выживших
        for name, rec in survivors.items():
            if rec.name != name:
                rec.name = name
        db.commit()
        seen = survivors
        print(f"[normalize_cities] города обновлены: {changed}, область проставлена: {regioned}, "
              f"городов в справочнике: {len(seen)}")
    finally:
        db.close()


if __name__ == "__main__":
    main()
