"""Привести телефоны учеников к формату +7XXXXXXXXXX (E.164).

Несколько номеров в одной ячейке (через пробел/запятую) нормализуются каждый и
сохраняются через запятую. Нераспознанное оставляем как есть.

    python -m scripts.normalize_phones
"""
import re

from app.core.database import SessionLocal
from app.models import Disciple


def norm_token(tok: str) -> str:
    d = re.sub(r"\D", "", tok)
    if len(d) == 11 and d[0] in "78":
        return "+7" + d[1:]
    if len(d) == 10:
        return "+7" + d
    return tok.strip()


def normalize(raw: str) -> str:
    parts = [p for p in re.split(r"[,\s;]+", raw.strip()) if p]
    out, seen = [], set()
    for p in parts:
        n = norm_token(p)
        if n and n not in seen:
            seen.add(n)
            out.append(n)
    return ", ".join(out)


def main():
    db = SessionLocal()
    changed = 0
    try:
        for d in db.query(Disciple).filter(Disciple.phone.isnot(None)).all():
            new = normalize(d.phone)
            if new != d.phone:
                d.phone = new
                changed += 1
        db.commit()
        print(f"[normalize_phones] обновлено: {changed}")
    finally:
        db.close()


if __name__ == "__main__":
    main()
