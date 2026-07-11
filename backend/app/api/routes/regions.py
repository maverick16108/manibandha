from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, staff_user
from app.core.database import get_db
from app.models import Region, User
from app.schemas.region import RegionCreate, RegionOut, RegionUpdate

router = APIRouter(prefix="/regions", tags=["regions"])


@router.get("", response_model=list[RegionOut])
def list_regions(db: Session = Depends(get_db), _: User = Depends(get_current_user)):
    return db.query(Region).order_by(Region.name).all()


@router.post("", response_model=RegionOut, status_code=status.HTTP_201_CREATED)
def create_region(payload: RegionCreate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    name = payload.name.strip()
    if not name:
        raise HTTPException(status_code=400, detail="Название региона обязательно")
    if db.query(Region).filter(Region.name.ilike(name)).first():
        raise HTTPException(status_code=400, detail="Такой регион уже есть")
    region = Region(name=name)
    db.add(region)
    db.commit()
    db.refresh(region)
    return region


@router.patch("/{region_id}", response_model=RegionOut)
def update_region(region_id: int, payload: RegionUpdate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    region = db.get(Region, region_id)
    if not region:
        raise HTTPException(status_code=404, detail="Регион не найден")
    for k, v in payload.model_dump(exclude_unset=True).items():
        setattr(region, k, v)
    db.commit()
    db.refresh(region)
    return region


@router.delete("/{region_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_region(region_id: int, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    region = db.get(Region, region_id)
    if not region:
        raise HTTPException(status_code=404, detail="Регион не найден")
    db.delete(region)
    db.commit()
