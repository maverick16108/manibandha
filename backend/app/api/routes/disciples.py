from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy import or_
from sqlalchemy.orm import Session, joinedload

from app.api.deps import get_current_user, scope_disciple_query, staff_user
from app.core.database import get_db
from app.core.enums import InitiationStatus, Role
from app.models import Disciple, User
from app.schemas.disciple import (
    DiscipleCreate,
    DiscipleListResponse,
    DiscipleOut,
    DiscipleUpdate,
)

router = APIRouter(prefix="/disciples", tags=["disciples"])


def _load(db: Session):
    return db.query(Disciple).options(
        joinedload(Disciple.temple),
        joinedload(Disciple.mentor),
        joinedload(Disciple.checklist),
    )


@router.get("", response_model=DiscipleListResponse)
def list_disciples(
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
    q: str | None = Query(None, description="Поиск по имени (духовное/мирское)"),
    status_: InitiationStatus | None = Query(None, alias="status"),
    country: str | None = None,
    region: str | None = None,
    city: str | None = None,
    temple_id: int | None = None,
    mentor_id: int | None = None,
    ready: bool | None = None,
    ready_pranama: bool | None = None,
    sort: str = Query("material_name", description="material_name|spiritual_name|created_at|initiation_status"),
    skip: int = 0,
    limit: int = Query(50, le=500),
):
    query = scope_disciple_query(_load(db), user)

    if q:
        like = f"%{q.strip()}%"
        query = query.filter(or_(Disciple.material_name.ilike(like), Disciple.spiritual_name.ilike(like)))
    if status_:
        query = query.filter(Disciple.initiation_status == status_)
    if country:
        query = query.filter(Disciple.country.ilike(country))
    if region:
        query = query.filter(Disciple.region.ilike(region))
    if city:
        query = query.filter(Disciple.city.ilike(city))
    if temple_id:
        query = query.filter(Disciple.temple_id == temple_id)
    if mentor_id:
        query = query.filter(Disciple.mentor_id == mentor_id)
    if ready is not None:
        query = query.filter(Disciple.ready_for_initiation.is_(ready))
    if ready_pranama is not None:
        query = query.filter(Disciple.ready_for_pranama.is_(ready_pranama))

    total = query.count()

    sort_map = {
        "material_name": Disciple.material_name,
        "spiritual_name": Disciple.spiritual_name,
        "created_at": Disciple.created_at.desc(),
        "initiation_status": Disciple.initiation_status,
    }
    query = query.order_by(sort_map.get(sort, Disciple.material_name))

    items = query.offset(skip).limit(limit).all()
    return DiscipleListResponse(total=total, items=items)


def _get_scoped(db: Session, user: User, disciple_id: int) -> Disciple:
    disciple = scope_disciple_query(_load(db), user).filter(Disciple.id == disciple_id).first()
    if not disciple:
        raise HTTPException(status_code=404, detail="Ученик не найден")
    return disciple


@router.get("/{disciple_id}", response_model=DiscipleOut)
def get_disciple(disciple_id: int, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    return _get_scoped(db, user, disciple_id)


@router.post("", response_model=DiscipleOut, status_code=status.HTTP_201_CREATED)
def create_disciple(payload: DiscipleCreate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    disciple = Disciple(**payload.model_dump())
    db.add(disciple)
    db.commit()
    db.refresh(disciple)
    return _load(db).filter(Disciple.id == disciple.id).first()


@router.patch("/{disciple_id}", response_model=DiscipleOut)
def update_disciple(
    disciple_id: int, payload: DiscipleUpdate, db: Session = Depends(get_db), user: User = Depends(get_current_user)
):
    # Staff can edit anyone; curator can edit own; student can edit own card.
    if user.role == Role.curator:
        disciple = _get_scoped(db, user, disciple_id)
    elif user.role == Role.student:
        if user.disciple_id != disciple_id:
            raise HTTPException(status_code=403, detail="Можно редактировать только свою анкету")
        disciple = db.get(Disciple, disciple_id)
    elif user.role in (Role.guru, Role.secretary):
        disciple = db.get(Disciple, disciple_id)
    else:
        raise HTTPException(status_code=403, detail="Недостаточно прав")

    if not disciple:
        raise HTTPException(status_code=404, detail="Ученик не найден")

    for k, v in payload.model_dump(exclude_unset=True).items():
        setattr(disciple, k, v)
    db.commit()
    return _load(db).filter(Disciple.id == disciple_id).first()


@router.delete("/{disciple_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_disciple(disciple_id: int, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    disciple = db.get(Disciple, disciple_id)
    if not disciple:
        raise HTTPException(status_code=404, detail="Ученик не найден")
    db.delete(disciple)
    db.commit()
