import io
import os
from datetime import datetime

from fastapi import APIRouter, Depends, Query
from fastapi.responses import StreamingResponse
from openpyxl import Workbook
from reportlab.lib import colors
from reportlab.lib.pagesizes import A4, landscape
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from sqlalchemy import func
from sqlalchemy.orm import Session, joinedload

from app.api.deps import get_current_user, scope_disciple_query
from app.core.database import get_db
from app.core.enums import InitiationStatus
from app.models import Disciple, User
from app.schemas.report import CountByKey, ReportSummary

router = APIRouter(prefix="/reports", tags=["reports"])

STATUS_LABELS = {
    InitiationStatus.aspirant: "Аспирант",
    InitiationStatus.recommended: "Рекомендован",
    InitiationStatus.harinama: "Харинама",
    InitiationStatus.brahman: "Брахман",
}

# Register a Unicode (Cyrillic-capable) font for PDF if one is available.
_FONT_CANDIDATES = [
    "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
    "/usr/share/fonts/dejavu/DejaVuSans.ttf",
    "/Library/Fonts/Arial Unicode.ttf",
    "/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
    "/System/Library/Fonts/Supplemental/Times New Roman.ttf",
]
PDF_FONT = "Helvetica"
for _path in _FONT_CANDIDATES:
    if os.path.exists(_path):
        try:
            pdfmetrics.registerFont(TTFont("AppFont", _path))
            PDF_FONT = "AppFont"
            break
        except Exception:
            continue


def _apply_filters(q, status_, country, city, temple_id, mentor_id, ready, search):
    if status_:
        q = q.filter(Disciple.initiation_status == status_)
    if country:
        q = q.filter(Disciple.country.ilike(country))
    if city:
        q = q.filter(Disciple.city.ilike(city))
    if temple_id:
        q = q.filter(Disciple.temple_id == temple_id)
    if mentor_id:
        q = q.filter(Disciple.mentor_id == mentor_id)
    if ready is not None:
        q = q.filter(Disciple.ready_for_initiation.is_(ready))
    if search:
        like = f"%{search.strip()}%"
        q = q.filter(Disciple.material_name.ilike(like) | Disciple.spiritual_name.ilike(like))
    return q


def _filtered_query(db, user, status_, country, city, temple_id, mentor_id, ready, search):
    q = scope_disciple_query(
        db.query(Disciple).options(joinedload(Disciple.temple), joinedload(Disciple.mentor)), user
    )
    return _apply_filters(q, status_, country, city, temple_id, mentor_id, ready, search)


# group_by dimension -> (column, label resolver)
def _group_dimension(db, key: str):
    from app.models import Temple

    if key == "status":
        return Disciple.initiation_status, lambda v: STATUS_LABELS.get(v, str(v) if v else "—")
    if key == "country":
        return Disciple.country, lambda v: v or "—"
    if key == "city":
        return Disciple.city, lambda v: v or "—"
    if key == "mentor":
        names = {u.id: u.full_name for u in db.query(User).all()}
        return Disciple.mentor_id, lambda v: names.get(v, "—") if v else "—"
    # default: temple
    tnames = {t.id: t.name for t in db.query(Temple).all()}
    return Disciple.temple_id, lambda v: tnames.get(v, "—") if v else "—"


@router.get("/summary", response_model=ReportSummary)
def summary(db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    from app.models import Temple

    total = scope_disciple_query(db.query(Disciple), user).count()

    by_status_rows = (
        scope_disciple_query(db.query(Disciple.initiation_status, func.count()), user)
        .group_by(Disciple.initiation_status)
        .all()
    )
    by_status = [CountByKey(key=STATUS_LABELS.get(s, str(s)), count=c) for s, c in by_status_rows]

    by_country_rows = (
        scope_disciple_query(db.query(Disciple.country, func.count()), user)
        .group_by(Disciple.country)
        .order_by(func.count().desc())
        .all()
    )
    by_country = [CountByKey(key=(c or "—"), count=n) for c, n in by_country_rows]

    by_temple_rows = (
        scope_disciple_query(db.query(Disciple.temple_id, func.count()), user)
        .group_by(Disciple.temple_id)
        .all()
    )
    temple_names = {t.id: t.name for t in db.query(Temple).all()}
    by_temple = [CountByKey(key=(temple_names.get(tid, "—") if tid else "—"), count=n) for tid, n in by_temple_rows]

    ready = scope_disciple_query(db.query(Disciple), user).filter(Disciple.ready_for_initiation.is_(True)).count()

    return ReportSummary(
        total=total, by_status=by_status, by_country=by_country, by_temple=by_temple, ready_for_initiation=ready
    )


@router.get("/group", response_model=list[CountByKey])
def group(
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
    group_by: str = Query("status", description="status|country|city|temple|mentor"),
    status_: InitiationStatus | None = Query(None, alias="status"),
    country: str | None = None,
    city: str | None = None,
    temple_id: int | None = None,
    mentor_id: int | None = None,
    ready: bool | None = None,
    q: str | None = None,
):
    col, label = _group_dimension(db, group_by)
    query = scope_disciple_query(db.query(col, func.count()), user)
    query = _apply_filters(query, status_, country, city, temple_id, mentor_id, ready, q)
    rows = query.group_by(col).order_by(func.count().desc()).all()
    return [CountByKey(key=label(v), count=n) for v, n in rows]


_COLUMNS = [
    ("Духовное имя", lambda d: d.spiritual_name or ""),
    ("Мирское имя", lambda d: d.material_name or ""),
    ("Статус", lambda d: STATUS_LABELS.get(d.initiation_status, "")),
    ("Страна", lambda d: d.country or ""),
    ("Город", lambda d: d.city or ""),
    ("Храм", lambda d: d.temple.name if d.temple else ""),
    ("Наставник", lambda d: d.mentor.full_name if d.mentor else ""),
    ("Телефон", lambda d: d.phone or ""),
    ("Email", lambda d: d.email or ""),
]


@router.get("/disciples.xlsx")
def export_xlsx(
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
    status_: InitiationStatus | None = Query(None, alias="status"),
    country: str | None = None,
    city: str | None = None,
    temple_id: int | None = None,
    mentor_id: int | None = None,
    ready: bool | None = None,
    q: str | None = None,
):
    rows = (
        _filtered_query(db, user, status_, country, city, temple_id, mentor_id, ready, q)
        .order_by(Disciple.material_name)
        .all()
    )

    wb = Workbook()
    ws = wb.active
    ws.title = "Ученики"
    ws.append([c[0] for c in _COLUMNS])
    for d in rows:
        ws.append([fn(d) for _, fn in _COLUMNS])
    for i, _ in enumerate(_COLUMNS, start=1):
        ws.column_dimensions[ws.cell(row=1, column=i).column_letter].width = 22

    buf = io.BytesIO()
    wb.save(buf)
    buf.seek(0)
    fname = f"disciples_{datetime.now():%Y%m%d}.xlsx"
    return StreamingResponse(
        buf,
        media_type="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        headers={"Content-Disposition": f'attachment; filename="{fname}"'},
    )


@router.get("/disciples.pdf")
def export_pdf(
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
    status_: InitiationStatus | None = Query(None, alias="status"),
    country: str | None = None,
    city: str | None = None,
    temple_id: int | None = None,
    mentor_id: int | None = None,
    ready: bool | None = None,
    q: str | None = None,
):
    rows = (
        _filtered_query(db, user, status_, country, city, temple_id, mentor_id, ready, q)
        .order_by(Disciple.material_name)
        .all()
    )

    buf = io.BytesIO()
    doc = SimpleDocTemplate(buf, pagesize=landscape(A4), leftMargin=12 * mm, rightMargin=12 * mm,
                            topMargin=12 * mm, bottomMargin=12 * mm)
    styles = getSampleStyleSheet()
    title_style = ParagraphStyle("t", parent=styles["Title"], fontName=PDF_FONT, fontSize=16)
    cell_style = ParagraphStyle("c", parent=styles["Normal"], fontName=PDF_FONT, fontSize=8, leading=10)

    elements = [
        Paragraph("Список учеников — Manibandha", title_style),
        Paragraph(f"Сформировано: {datetime.now():%d.%m.%Y %H:%M} · всего: {len(rows)}", cell_style),
        Spacer(1, 6 * mm),
    ]

    header = [Paragraph(f"<b>{c[0]}</b>", cell_style) for c in _COLUMNS]
    data = [header]
    for d in rows:
        data.append([Paragraph(str(fn(d)), cell_style) for _, fn in _COLUMNS])

    table = Table(data, repeatRows=1)
    table.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#7c2d12")),
                ("TEXTCOLOR", (0, 0), (-1, 0), colors.white),
                ("GRID", (0, 0), (-1, -1), 0.25, colors.HexColor("#d6c3a5")),
                ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, colors.HexColor("#faf6f0")]),
                ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
                ("FONTNAME", (0, 0), (-1, -1), PDF_FONT),
            ]
        )
    )
    elements.append(table)
    doc.build(elements)
    buf.seek(0)

    fname = f"disciples_{datetime.now():%Y%m%d}.pdf"
    return StreamingResponse(
        buf, media_type="application/pdf", headers={"Content-Disposition": f'attachment; filename="{fname}"'}
    )
