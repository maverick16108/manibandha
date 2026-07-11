from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, staff_user
from app.core.database import get_db
from app.models import Temple, User
from app.schemas.temple import TempleCreate, TempleOut, TempleUpdate

router = APIRouter(prefix="/temples", tags=["temples"])


@router.get("", response_model=list[TempleOut])
def list_temples(db: Session = Depends(get_db), _: User = Depends(get_current_user)):
    return db.query(Temple).order_by(Temple.name).all()


@router.post("", response_model=TempleOut, status_code=status.HTTP_201_CREATED)
def create_temple(payload: TempleCreate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    temple = Temple(**payload.model_dump())
    db.add(temple)
    db.commit()
    db.refresh(temple)
    return temple


@router.patch("/{temple_id}", response_model=TempleOut)
def update_temple(temple_id: int, payload: TempleUpdate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    temple = db.get(Temple, temple_id)
    if not temple:
        raise HTTPException(status_code=404, detail="Храм не найден")
    for k, v in payload.model_dump(exclude_unset=True).items():
        setattr(temple, k, v)
    db.commit()
    db.refresh(temple)
    return temple


@router.delete("/{temple_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_temple(temple_id: int, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    temple = db.get(Temple, temple_id)
    if not temple:
        raise HTTPException(status_code=404, detail="Храм не найден")
    db.delete(temple)
    db.commit()
