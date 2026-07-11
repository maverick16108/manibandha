from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, staff_user
from app.core.database import get_db
from app.models import City, User
from app.schemas.city import CityCreate, CityOut, CityUpdate

router = APIRouter(prefix="/cities", tags=["cities"])


@router.get("", response_model=list[CityOut])
def list_cities(db: Session = Depends(get_db), _: User = Depends(get_current_user)):
    return db.query(City).order_by(City.name).all()


@router.post("", response_model=CityOut, status_code=status.HTTP_201_CREATED)
def create_city(payload: CityCreate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    name = payload.name.strip()
    if not name:
        raise HTTPException(status_code=400, detail="Название города обязательно")
    if db.query(City).filter(City.name.ilike(name)).first():
        raise HTTPException(status_code=400, detail="Такой город уже есть")
    city = City(name=name, country=(payload.country or None))
    db.add(city)
    db.commit()
    db.refresh(city)
    return city


@router.patch("/{city_id}", response_model=CityOut)
def update_city(city_id: int, payload: CityUpdate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    city = db.get(City, city_id)
    if not city:
        raise HTTPException(status_code=404, detail="Город не найден")
    for k, v in payload.model_dump(exclude_unset=True).items():
        setattr(city, k, v)
    db.commit()
    db.refresh(city)
    return city


@router.delete("/{city_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_city(city_id: int, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    city = db.get(City, city_id)
    if not city:
        raise HTTPException(status_code=404, detail="Город не найден")
    db.delete(city)
    db.commit()
