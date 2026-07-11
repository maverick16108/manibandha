from typing import Iterable

import jwt
from fastapi import Depends, HTTPException, status
from fastapi.security import OAuth2PasswordBearer
from sqlalchemy.orm import Session

from app.core.database import get_db
from app.core.enums import Role
from app.core.security import decode_access_token
from app.models import User

oauth2_scheme = OAuth2PasswordBearer(tokenUrl="api/auth/login")

_credentials_exc = HTTPException(
    status_code=status.HTTP_401_UNAUTHORIZED,
    detail="Не удалось проверить учётные данные",
    headers={"WWW-Authenticate": "Bearer"},
)


def get_current_user(token: str = Depends(oauth2_scheme), db: Session = Depends(get_db)) -> User:
    try:
        payload = decode_access_token(token)
        email = payload.get("sub")
        if not email:
            raise _credentials_exc
    except jwt.PyJWTError:
        raise _credentials_exc

    user = db.query(User).filter(User.email == email).first()
    if user is None or not user.is_active:
        raise _credentials_exc
    return user


def require_roles(*roles: Role):
    allowed = set(roles)

    def _checker(user: User = Depends(get_current_user)) -> User:
        if user.role not in allowed:
            raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Недостаточно прав")
        return user

    return _checker


# Convenience role bundles
def staff_user(user: User = Depends(get_current_user)) -> User:
    """Guru or secretary — full management access."""
    if user.role not in (Role.guru, Role.secretary):
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Недостаточно прав")
    return user


def can_view_disciples(user: User = Depends(get_current_user)) -> User:
    """Everyone authenticated may reach disciple endpoints; per-object scoping is
    applied inside the route (curator sees own, student sees self)."""
    return user


def is_guru(user: User) -> bool:
    return user.role == Role.guru


def scope_disciple_query(query, user: User):
    """Restrict a disciples query to what the given user is allowed to see."""
    if user.role in (Role.guru, Role.secretary):
        return query
    if user.role == Role.curator:
        from app.models import Disciple
        # наставник видит учеников, чей наставник — его анкета
        return query.filter(Disciple.mentor_id == (user.disciple_id or -1))
    if user.role == Role.student:
        from app.models import Disciple
        return query.filter(Disciple.id == (user.disciple_id or -1))
    return query.filter(False)
