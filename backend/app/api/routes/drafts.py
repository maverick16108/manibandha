from fastapi import APIRouter, Body, Depends, status
from sqlalchemy.orm import Session

from app.api.deps import get_current_user
from app.core.database import get_db
from app.models import Draft, User

router = APIRouter(prefix="/drafts", tags=["drafts"])


@router.get("/{scope}")
def get_draft(scope: str, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    row = db.query(Draft).filter(Draft.user_id == user.id, Draft.scope == scope).first()
    return {"body": row.body if row else ""}


@router.put("/{scope}")
def save_draft(scope: str, body: str = Body("", embed=True), db: Session = Depends(get_db),
               user: User = Depends(get_current_user)):
    row = db.query(Draft).filter(Draft.user_id == user.id, Draft.scope == scope).first()
    if row:
        row.body = body
    else:
        db.add(Draft(user_id=user.id, scope=scope, body=body))
    db.commit()
    return {"body": body}


@router.delete("/{scope}", status_code=status.HTTP_204_NO_CONTENT)
def delete_draft(scope: str, db: Session = Depends(get_db), user: User = Depends(get_current_user)):
    db.query(Draft).filter(Draft.user_id == user.id, Draft.scope == scope).delete()
    db.commit()
