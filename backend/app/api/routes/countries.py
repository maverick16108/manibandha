from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, staff_user
from app.core.database import get_db
from app.models import Country, User
from app.schemas.country import CountryCreate, CountryOut, CountryUpdate

router = APIRouter(prefix="/countries", tags=["countries"])


@router.get("", response_model=list[CountryOut])
def list_countries(db: Session = Depends(get_db), _: User = Depends(get_current_user)):
    return db.query(Country).order_by(Country.name).all()


@router.post("", response_model=CountryOut, status_code=status.HTTP_201_CREATED)
def create_country(payload: CountryCreate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    name = payload.name.strip()
    if not name:
        raise HTTPException(status_code=400, detail="Название страны обязательно")
    if db.query(Country).filter(Country.name.ilike(name)).first():
        raise HTTPException(status_code=400, detail="Такая страна уже есть")
    country = Country(name=name)
    db.add(country)
    db.commit()
    db.refresh(country)
    return country


@router.patch("/{country_id}", response_model=CountryOut)
def update_country(country_id: int, payload: CountryUpdate, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    country = db.get(Country, country_id)
    if not country:
        raise HTTPException(status_code=404, detail="Страна не найдена")
    for k, v in payload.model_dump(exclude_unset=True).items():
        setattr(country, k, v)
    db.commit()
    db.refresh(country)
    return country


@router.delete("/{country_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_country(country_id: int, db: Session = Depends(get_db), _: User = Depends(staff_user)):
    country = db.get(Country, country_id)
    if not country:
        raise HTTPException(status_code=404, detail="Страна не найдена")
    db.delete(country)
    db.commit()
