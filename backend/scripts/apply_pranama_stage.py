"""Проставить стадию «Пранама-мантра».

1) Допарсить дату пранамы из примечаний, включая словесные месяцы
   («12 февраля 2026», «январь 2026») -> pranama_date.
2) Тем, у кого есть pranama_date и статус ещё «Кандидат» (не инициированы),
   выставить статус «pranama».

    python -m scripts.apply_pranama_stage
"""
import re
from datetime import date

from app.core.database import SessionLocal
from app.core.enums import InitiationStatus
from app.models import Disciple

# префикс месяца -> номер (длинные раньше коротких, чтобы «мар» не путался с «ма»)
_MONTHS = [
    ("янв", 1), ("фев", 2), ("март", 3), ("мар", 3), ("апр", 4), ("мая", 5), ("май", 5),
    ("июн", 6), ("июл", 7), ("авг", 8), ("сен", 9), ("окт", 10), ("ноя", 11), ("дек", 12),
]


def parse_ru_date(s: str):
    s = (s or "").strip().lower()
    m = re.search(r"(\d{1,2})[.\s]+(\d{1,2})[.\s]+(\d{2,4})", s)
    if m:
        d, mo, y = int(m.group(1)), int(m.group(2)), int(m.group(3))
        y += 2000 if y < 100 else 0
        try:
            return date(y, mo, d)
        except ValueError:
            return None
    m = re.search(r"(?:(\d{1,2})\s+)?([а-яё]+)\s+(\d{4})", s)
    if m:
        day = int(m.group(1)) if m.group(1) else 1
        word, year = m.group(2), int(m.group(3))
        for pref, num in _MONTHS:
            if word.startswith(pref):
                try:
                    return date(year, num, day)
                except ValueError:
                    return None
    return None


def main():
    db = SessionLocal()
    parsed = staged = 0
    try:
        for d in db.query(Disciple).all():
            # 1) добрать дату из примечаний
            if not d.pranama_date and d.notes:
                keep = []
                for line in d.notes.split("\n"):
                    m = re.match(r"^Пранама-мантра:\s*(.+)$", line.strip())
                    if m and not d.pranama_date:
                        dt = parse_ru_date(m.group(1))
                        if dt:
                            d.pranama_date = dt
                            parsed += 1
                            continue
                    keep.append(line)
                d.notes = "\n".join(keep).strip() or None
            # 2) выставить стадию
            if d.pranama_date and d.initiation_status == InitiationStatus.aspirant:
                d.initiation_status = InitiationStatus.pranama
                staged += 1
        db.commit()
        print(f"[apply_pranama_stage] дат добрано: {parsed}, статус «Пранама» выставлен: {staged}")
    finally:
        db.close()


if __name__ == "__main__":
    main()
