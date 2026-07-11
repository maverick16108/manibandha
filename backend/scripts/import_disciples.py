"""Импорт учеников из CSV-выгрузки Google-таблицы.

Ожидаемые колонки:
  № пп, ФИО, «духовное имя и дата инициации», город, дата рождения,
  служение, телефон, дата получения пранама-мантры, коммент по инициации

Использование:
    python -m scripts.import_disciples /path/to/sheet.csv
Идемпотентно: ученик с таким же мирским именем (ФИО) повторно не создаётся.
"""
import csv
import re
import sys
from datetime import date

from app.core.database import SessionLocal
from app.core.enums import InitiationStatus
from app.models import Disciple

DEFAULT_COUNTRY = "Россия"


def parse_date(raw: str):
    """'12.08.2004' | '8.01.26' | '20.05. 1983' -> date | None."""
    if not raw:
        return None
    m = re.search(r"(\d{1,2})\.\s*(\d{1,2})\.\s*(\d{2,4})", raw)
    if not m:
        return None
    d, mo, y = int(m.group(1)), int(m.group(2)), int(m.group(3))
    if y < 100:
        y += 2000
    try:
        return date(y, mo, d)
    except ValueError:
        return None


def parse_spiritual(raw: str):
    """'Самба дас (8.01.26)' -> ('Самба дас', date(2026,1,8))."""
    raw = (raw or "").strip()
    if not raw:
        return None, None
    dt = parse_date(raw)
    name = re.sub(r"\(.*?\)", "", raw).strip()
    return (name or None), dt


def main(path: str):
    db = SessionLocal()
    created = skipped = 0
    try:
        with open(path, encoding="utf-8") as f:
            reader = csv.DictReader(f)
            for row in reader:
                fio = (row.get("ФИО") or "").strip()
                if not fio:
                    continue
                if db.query(Disciple).filter(Disciple.material_name == fio).first():
                    skipped += 1
                    continue

                sp_name, hari_date = parse_spiritual(row.get("духовное имя и дата инициации"))
                status = InitiationStatus.harinama if sp_name else InitiationStatus.aspirant

                dob_raw = (row.get("дата рождения") or "").strip()
                dob = parse_date(dob_raw)

                # notes: pranama-mantra date, comment, birth-year-if-unparsed
                notes = []
                pranama = (row.get("дата получения пранама-мантры") or "").strip()
                if pranama:
                    notes.append(f"Пранама-мантра: {pranama}")
                comment = (row.get("коммет по инициации") or "").strip()
                if comment:
                    notes.append(f"Комментарий: {comment}")
                if dob_raw and not dob:
                    notes.append(f"Дата рождения (как в таблице): {dob_raw}")

                city = (row.get("город") or "").strip() or None

                db.add(Disciple(
                    material_name=fio,
                    spiritual_name=sp_name,
                    initiation_status=status,
                    harinama_date=hari_date,
                    city=city,
                    country=DEFAULT_COUNTRY if city else None,
                    date_of_birth=dob,
                    seva=(row.get("служение") or "").strip() or None,
                    phone=(row.get("телефон") or "").strip() or None,
                    notes="\n".join(notes) or None,
                ))
                created += 1
        db.commit()
        print(f"[import] создано: {created}, пропущено (уже есть): {skipped}")
    finally:
        db.close()


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise SystemExit("usage: python -m scripts.import_disciples <sheet.csv>")
    main(sys.argv[1])
