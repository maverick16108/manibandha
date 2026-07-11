from fastapi import APIRouter, Body, Depends
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, require_roles
from app.core import permissions as perm
from app.core.database import get_db
from app.core.enums import Role
from app.models import User

router = APIRouter(prefix="/permissions", tags=["permissions"])


@router.get("/me")
def my_permissions(db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    """Разделы, доступные текущему пользователю (для навигации фронта)."""
    return {"role": user.role.value, "sections": perm.role_sections(db, user.role)}


@router.get("")
def get_matrix(db: Session = Depends(get_db), _: User = Depends(require_roles(Role.guru))):
    return {"sections": perm.SECTIONS, "matrix": perm.full_matrix(db)}


@router.put("")
def put_matrix(
    matrix: dict = Body(..., embed=True),
    db: Session = Depends(get_db),
    _: User = Depends(require_roles(Role.guru)),
):
    perm.save_matrix(db, matrix)
    return perm.full_matrix(db)
