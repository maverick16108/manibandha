"""WebSocket для интерактивного чата в ветках: мгновенная доставка + «печатает…».

Работает с одним воркером uvicorn (менеджер соединений хранится в процессе).
"""
from collections import defaultdict

import jwt
from fastapi import APIRouter, Query, WebSocket, WebSocketDisconnect
from sqlalchemy import func

from app.api.routes.threads import _accessible, _mark_read, _mark_staff_seen
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
                    reply_to_id = None
                    rid = data.get("reply_to_id")
                    if rid:
                        parent = db.get(ThreadMessage, rid)
                        if parent and parent.thread_id == thread_id:
                            reply_to_id = parent.id
                    msg = ThreadMessage(thread_id=thread_id, author_id=user.id, body=body, reply_to_id=reply_to_id)
                    db.add(msg)
                    t = db.get(Thread, thread_id)
                    t.updated_at = func.now()
                    _mark_staff_seen(db, user, t)
                    db.commit()
                    db.refresh(msg)
                    _mark_read(db, user, thread_id)
                    from app.api.routes.threads import _reply_dict
                    payload = {
                        "type": "message",
                        "message": {
                            "id": msg.id, "author_id": user.id, "author_name": user.full_name,
                            "body": msg.body, "created_at": msg.created_at.isoformat(),
                            "edit_count": 0, "reactions": [], "reply_to": _reply_dict(msg),
                        },
                    }
                await manager.broadcast(thread_id, payload)
    except WebSocketDisconnect:
        manager.rooms[thread_id].discard(websocket)
    except Exception:
        manager.rooms[thread_id].discard(websocket)
