"""WebSocket для интерактивного чата в ветках: мгновенная доставка + «печатает…».

Работает с одним воркером uvicorn (менеджер соединений хранится в процессе).
"""
from collections import defaultdict

import jwt
from fastapi import APIRouter, Query, WebSocket, WebSocketDisconnect
from sqlalchemy import func

from app.api.routes.threads import _accessible, _mark_read
from app.core.database import SessionLocal
from app.core.security import decode_access_token
from app.models import Thread, ThreadMessage, User

router = APIRouter()


class Manager:
    def __init__(self):
        self.rooms: dict[int, set[WebSocket]] = defaultdict(set)

    async def broadcast(self, thread_id: int, data: dict):
        for ws in list(self.rooms[thread_id]):
            try:
                await ws.send_json(data)
            except Exception:
                self.rooms[thread_id].discard(ws)


manager = Manager()


def _auth(token: str) -> User | None:
    try:
        email = decode_access_token(token).get("sub")
    except jwt.PyJWTError:
        return None
    if not email:
        return None
    with SessionLocal() as db:
        return db.query(User).filter(User.email == email).first()


@router.websocket("/ws/threads/{thread_id}")
async def ws_thread(websocket: WebSocket, thread_id: int, token: str = Query(...)):
    user = _auth(token)
    if not user or not user.is_active:
        await websocket.close(code=1008)
        return
    with SessionLocal() as db:
        if not _accessible(db, user).filter(Thread.id == thread_id).first():
            await websocket.close(code=1008)
            return

    await websocket.accept()
    manager.rooms[thread_id].add(websocket)
    try:
        while True:
            data = await websocket.receive_json()
            typ = data.get("type")
            if typ == "typing":
                await manager.broadcast(thread_id, {"type": "typing", "user_id": user.id, "name": user.full_name})
            elif typ == "message":
                body = (data.get("body") or "").strip()
                if not body:
                    continue
                with SessionLocal() as db:
                    if not _accessible(db, user).filter(Thread.id == thread_id).first():
                        continue
                    msg = ThreadMessage(thread_id=thread_id, author_id=user.id, body=body)
                    db.add(msg)
                    t = db.get(Thread, thread_id)
                    t.updated_at = func.now()
                    db.commit()
                    db.refresh(msg)
                    _mark_read(db, user, thread_id)
                    payload = {
                        "type": "message",
                        "message": {
                            "id": msg.id, "author_id": user.id, "author_name": user.full_name,
                            "body": msg.body, "created_at": msg.created_at.isoformat(),
                            "likes": 0, "liked": False,
                        },
                    }
                await manager.broadcast(thread_id, payload)
    except WebSocketDisconnect:
        manager.rooms[thread_id].discard(websocket)
    except Exception:
        manager.rooms[thread_id].discard(websocket)
