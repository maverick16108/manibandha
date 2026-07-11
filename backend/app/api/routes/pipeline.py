from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, scope_disciple_query, staff_user
from app.core.database import get_db
from app.core.enums import Role
from app.models import ChecklistItem, Disciple, User
from app.schemas.checklist import ChecklistItemCreate, ChecklistItemOut, ChecklistItemUpdate

router = APIRouter(prefix="/disciples/{disciple_id}/checklist", tags=["pipeline"])


def _ensure_access(db: Session, user: User, disciple_id: int) -> Disciple:
    disciple = scope_disciple_query(db.query(Disciple), user).filter(Disciple.id == disciple_id).first()
    if not disciple:
        raise HTTPException(status_code=404, detail="Ученик не найден")
    return disciple


def _can_edit(user: User) -> bool:
    return user.role in (Role.guru, Role.secretary, Role.curator)


@router.get("", response_model=list[ChecklistItemOut])
def list_items(disciple_id: int, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    _ensure_access(db, user, disciple_id)
    return (
        db.query(ChecklistItem)
        .filter(ChecklistItem.disciple_id == disciple_id)
        .order_by(ChecklistItem.id)
        .all()
    )


@router.post("", response_model=ChecklistItemOut, status_code=status.HTTP_201_CREATED)
def add_item(
    disciple_id: int, payload: ChecklistItemCreate, db: Session = Depends(get_db), user: User = Depends(get_current_user)
):
    if not _can_edit(user):
        raise HTTPException(status_code=403, detail="Недостаточно прав")
    _ensure_access(db, user, disciple_id)
    item = ChecklistItem(disciple_id=disciple_id, **payload.model_dump())
    db.add(item)
    db.commit()
    db.refresh(item)
    return item


@router.patch("/{item_id}", response_model=ChecklistItemOut)
def update_item(
    disciple_id: int,
    item_id: int,
    payload: ChecklistItemUpdate,
    db: Session = Depends(get_db),
    user: User = Depends(get_current_user),
):
    if not _can_edit(user):
        raise HTTPException(status_code=403, detail="Недостаточно прав")
    _ensure_access(db, user, disciple_id)
    item = db.get(ChecklistItem, item_id)
    if not item or item.disciple_id != disciple_id:
        raise HTTPException(status_code=404, detail="Пункт не найден")
    for k, v in payload.model_dump(exclude_unset=True).items():
        setattr(item, k, v)
    db.commit()
    db.refresh(item)
    return item


@router.delete("/{item_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_item(disciple_id: int, item_id: int, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    item = db.get(ChecklistItem, item_id)
    if not item or item.disciple_id != disciple_id:
        raise HTTPException(status_code=404, detail="Пункт не найден")
    db.delete(item)
    db.commit()
