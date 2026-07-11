import secrets
from datetime import datetime, timedelta, timezone

from fastapi import APIRouter, Body, Depends, HTTPException, status
from fastapi.security import OAuth2PasswordRequestForm
from sqlalchemy.orm import Session

from app.api.deps import get_current_user
from app.core.config import settings
from app.core.database import get_db
from app.core.enums import InitiationStatus, Role, ThreadKind
from app.core.security import create_access_token, hash_password, verify_password
from app.core.sms import normalize_phone, send_sms
from app.models import Disciple, SmsCode, Thread, User
from app.schemas.auth import Token
from app.schemas.user import SelfUpdate, UserOut

router = APIRouter(prefix="/auth", tags=["auth"])


def _token_for(user: User) -> Token:
    return Token(access_token=create_access_token(subject=user.email, role=user.role.value))


@router.post("/phone/request")
def phone_request(phone: str = Body(...), purpose: str = Body("auto"), db: Session = Depends(get_db)):
    """Отправить SMS-код на телефон. purpose: 'login' | 'register' | 'auto'.
    SMS не отправляется, если номер не подходит под выбранное действие."""
    ph = normalize_phone(phone)
    if len(ph) != 11:
        raise HTTPException(status_code=400, detail="Некорректный номер телефона")
    exists = db.query(User).filter(User.phone == ph).first() is not None
    if purpose == "register" and exists:
        raise HTTPException(status_code=400, detail="Этот номер уже зарегистрирован — перейдите на «Вход».")
    if purpose == "login" and not exists:
        raise HTTPException(status_code=400, detail="Этот номер не зарегистрирован — перейдите на «Регистрация».")
    code = f"{secrets.randbelow(10000):04d}"
    db.query(SmsCode).filter(SmsCode.phone == ph).delete()
    db.add(SmsCode(
        phone=ph, code=code,
        expires_at=datetime.now(timezone.utc) + timedelta(seconds=settings.SMS_CODE_TTL_SECONDS),
    ))
    db.commit()
    send_sms(ph, f"Код для входа на manibandha.ru: {code}")
    return {"sent": True, "exists": exists}


@router.post("/phone/verify", response_model=Token)
def phone_verify(phone: str = Body(...), code: str = Body(...), db: Session = Depends(get_db)):
    """Проверить код: существующий телефон → вход, новый → регистрация (создаётся анкета)."""
    ph = normalize_phone(phone)
    rec = db.query(SmsCode).filter(SmsCode.phone == ph).order_by(SmsCode.id.desc()).first()
    if not rec or rec.expires_at < datetime.now(timezone.utc):
        raise HTTPException(status_code=400, detail="Код истёк, запросите новый")
    rec.attempts += 1
    if rec.attempts > 5:
        db.query(SmsCode).filter(SmsCode.phone == ph).delete()
        db.commit()
        raise HTTPException(status_code=400, detail="Слишком много попыток, запросите новый код")
    if rec.code != code.strip():
        db.commit()
        raise HTTPException(status_code=400, detail="Неверный код")

    db.query(SmsCode).filter(SmsCode.phone == ph).delete()
    user = db.query(User).filter(User.phone == ph).first()
    if user:
        db.commit()
        return _token_for(user)

    # регистрация: создаём связанную пару пользователь + анкета (ждёт апрува).
    # Имя пустое — кандидат заполнит сам; телефон сохраняем для идентификации.
    disciple = Disciple(
        material_name="", phone=f"+{ph}",
        initiation_status=InitiationStatus.recommended, is_approved=False,
    )
    db.add(disciple)
    db.flush()
    user = User(
        email=f"{ph}@phone.local", phone=ph, hashed_password=hash_password(secrets.token_urlsafe(16)),
        full_name=f"+{ph}", role=Role.student, disciple_id=disciple.id,
    )
    db.add(user)
    db.add(Thread(kind=ThreadKind.approval, disciple_id=disciple.id))
    db.commit()
    return _token_for(user)


@router.post("/login", response_model=Token)
def login(form: OAuth2PasswordRequestForm = Depends(), db: Session = Depends(get_db)):
    # OAuth2PasswordRequestForm uses `username`; we treat it as email.
    user = db.query(User).filter(User.email == form.username).first()
    if not user or not verify_password(form.password, user.hashed_password):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Неверный email или пароль")
    if not user.is_active:
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Учётная запись отключена")
    token = create_access_token(subject=user.email, role=user.role.value)
    return Token(access_token=token)


@router.get("/me", response_model=UserOut)
def me(user: User = Depends(get_current_user)):
    return user


@router.patch("/me", response_model=UserOut)
def update_me(payload: SelfUpdate, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    data = payload.model_dump(exclude_unset=True)
    for k, v in data.items():
        setattr(user, k, v)
    db.commit()
    db.refresh(user)
    return user
