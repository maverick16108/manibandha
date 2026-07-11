"""Перенести дату пранама-мантры из примечаний в поле pranama_date.

При импорте таблицы дата пранама-мантры сохранялась в примечаниях строкой
«Пранама-мантра: <значение>». Здесь разбираем те, что являются датой (дд.мм.гггг),
переносим в pranama_date и убираем строку из примечаний. Текстовые («январь 2026»,
«читает», «осень 2025») остаются в примечаниях как есть.

    python -m scripts.extract_pranama
"""
import re

from app.core.database import SessionLocal
from app.models import Disciple
from scripts.import_disciples import parse_date


def main():
    db = SessionLocal()
    moved = 0
    try:
        for d in db.query(Disciple).filter(Disciple.notes.isnot(None)).all():
            lines = d.notes.split("\n")
            keep = []
            for line in lines:
                m = re.match(r"^Пранама-мантра:\s*(.+)$", line.strip())
                if m and not d.pranama_date:
                    dt = parse_date(m.group(1))
                    if dt:
                        d.pranama_date = dt
                        moved += 1
                        continue  # строку не сохраняем — дата теперь в поле
                keep.append(line)
            new_notes = "\n".join(keep).strip()
            d.notes = new_notes or None
        db.commit()
        print(f"[extract_pranama] дат перенесено: {moved}")
    finally:
        db.close()


if __name__ == "__main__":
    main()
