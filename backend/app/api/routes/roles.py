from fastapi import APIRouter, Body, Depends, HTTPException, status
from sqlalchemy.orm import Session

from app.api.deps import get_current_user, require_cap
from app.core.capabilities import CAP_KEYS, capabilities_grouped, user_capabilities
from app.core.database import get_db
from app.models import Role, User, UserRole

router = APIRouter(tags=["roles"])


def _role_out(r: Role) -> dict:
    return {
        "id": r.id, "key": r.key, "name": r.name,
        "is_system": r.is_system, "is_superadmin": r.is_superadmin, "is_default": r.is_default,
        "capabilities": list(r.capabilities or []),
    }


@router.get("/me/capabilities")
def my_capabilities(db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    """Права и роли текущего пользователя — для навигации/гейтинга на фронте."""
    from app.core.capabilities import user_roles as _ur
    return {
        "capabilities": sorted(user_capabilities(db, user)),
        "roles": [r.key for r in _ur(db, user)],
    }


@router.get("/capabilities")
def list_capabilities(_: User = Depends(require_cap("roles.manage"))):
    """Каталог всех прав (сгруппированный) — для редактора ролей."""
    return capabilities_grouped()


@router.get("/roles")
def list_roles(db: Session = Depends(get_db), _: User = Depends(require_cap("roles.manage"))):
    return [_role_out(r) for r in db.query(Role).order_by(Role.is_system.desc(), Role.id).all()]


@router.post("/roles", status_code=status.HTTP_201_CREATED)
def create_role(payload: dict = Body(...), db: Session = Depends(get_db), _: User = Depends(require_cap("roles.manage"))):
    key = (payload.get("key") or "").strip().lower().replace(" ", "_")
    name = (payload.get("name") or "").strip()
    if not name:
        raise HTTPException(status_code=400, detail="Укажите название роли")
    if not key:
        key = name.lower().replace(" ", "_")
    if db.query(Role).filter(Role.key == key).first():
        raise HTTPException(status_code=400, detail="Роль с таким ключом уже существует")
    caps = [c for c in (payload.get("capabilities") or []) if c in CAP_KEYS]
    role = Role(key=key, name=name, is_system=False, is_superadmin=False,
                is_default=bool(payload.get("is_default")), capabilities=caps)
    if role.is_default:
        db.query(Role).update({Role.is_default: False})
    db.add(role)
    db.commit()
    db.refresh(role)
    return _role_out(role)


@router.put("/roles/{role_id}")
def update_role(role_id: int, payload: dict = Body(...), db: Session = Depends(get_db),
                _: User = Depends(require_cap("roles.manage"))):
    role = db.get(Role, role_id)
    if not role:
        raise HTTPException(status_code=404, detail="Роль не найдена")
    if role.is_superadmin:
        raise HTTPException(status_code=400, detail="Роль гуру не редактируется")
    if "name" in payload and payload["name"].strip():
        role.name = payload["name"].strip()
    if "capabilities" in payload:
        role.capabilities = [c for c in payload["capabilities"] if c in CAP_KEYS]
    if "is_default" in payload:
        role.is_default = bool(payload["is_default"])
        if role.is_default:
            db.query(Role).filter(Role.id != role.id).update({Role.is_default: False})
    db.commit()
    db.refresh(role)
    return _role_out(role)


@router.delete("/roles/{role_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_role(role_id: int, db: Session = Depends(get_db), _: User = Depends(require_cap("roles.manage"))):
    role = db.get(Role, role_id)
    if not role:
        raise HTTPException(status_code=404, detail="Роль не найдена")
    if role.is_system:
        raise HTTPException(status_code=400, detail="Системную роль удалить нельзя")
    db.query(UserRole).filter(UserRole.role_id == role.id).delete()
    db.delete(role)
    db.commit()


@router.get("/users/{user_id}/roles")
def get_user_roles(user_id: int, db: Session = Depends(get_db), _: User = Depends(require_cap("users.manage"))):
    ids = [ur.role_id for ur in db.query(UserRole).filter(UserRole.user_id == user_id).all()]
    return {"role_ids": ids}


@router.put("/users/{user_id}/roles")
def set_user_roles(user_id: int, payload: dict = Body(...), db: Session = Depends(get_db),
                   _: User = Depends(require_cap("users.manage"))):
    user = db.get(User, user_id)
    if not user:
        raise HTTPException(status_code=404, detail="Пользователь не найден")
    role_ids = payload.get("role_ids") or []
    valid = {r.id for r in db.query(Role).filter(Role.id.in_(role_ids)).all()} if role_ids else set()
    db.query(UserRole).filter(UserRole.user_id == user_id).delete()
    for rid in valid:
        db.add(UserRole(user_id=user_id, role_id=rid))
    db.commit()
    return {"role_ids": sorted(valid)}
