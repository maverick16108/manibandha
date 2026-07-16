"""WebSocket мессенджера: один сокет на пользователя, realtime-доставка апдейтов.

Клиент отправляет сообщения через REST (POST /chats/{id}/messages — идемпотентно,
возвращает id+seq), а принимает чужие сообщения и события «печатает…» здесь.
Догон пропущенного при реконнекте — через GET /chats/updates?since={pts}.

Работает с одним воркером uvicorn (менеджер соединений — в памяти процесса).
"""
from collections import defaultdict

import jwt
from fastapi import APIRouter, Query, WebSocket, WebSocketDisconnect

from app.core.database import SessionLocal
from app.core.security import decode_access_token
from app.models import ChatMember, User

router = APIRouter()


class ChatManager:
    def __init__(self):
        self.sockets: dict[int, set[WebSocket]] = defaultdict(set)  # user_id -> sockets

    def add(self, user_id: int, ws: WebSocket):
        self.sockets[user_id].add(ws)

    def remove(self, user_id: int, ws: WebSocket):
        self.sockets[user_id].discard(ws)
        if not self.sockets[user_id]:
            self.sockets.pop(user_id, None)

    async def send_to_users(self, user_ids: list[int], data: dict):
        seen = set()
        for uid in user_ids:
            for ws in list(self.sockets.get(uid, ())):
                if ws in seen:
                    continue
                seen.add(ws)
                try:
                    await ws.send_json(data)
                except Exception:
                    self.sockets.get(uid, set()).discard(ws)


manager = ChatManager()


def _auth(token: str) -> User | None:
    try:
        email = decode_access_token(token).get("sub")
    except jwt.PyJWTError:
        return None
    if not email:
        return None
    with SessionLocal() as db:
        return db.query(User).filter(User.email == email).first()


def _member_ids(chat_id: int) -> list[int]:
    with SessionLocal() as db:
        return [m.user_id for m in db.query(ChatMember).filter(ChatMember.chat_id == chat_id).all()]


def _is_member(user_id: int, chat_id: int) -> bool:
    with SessionLocal() as db:
        return db.query(ChatMember).filter(
            ChatMember.chat_id == chat_id, ChatMember.user_id == user_id
        ).first() is not None


@router.websocket("/ws/chat")
async def ws_chat(websocket: WebSocket, token: str = Query(...)):
    user = _auth(token)
    if not user or not user.is_active:
        await websocket.close(code=1008)
        return
    await websocket.accept()
    manager.add(user.id, websocket)
    try:
        while True:
            data = await websocket.receive_json()
            typ = data.get("type")
            if typ == "typing":
                chat_id = data.get("chat_id")
                if not isinstance(chat_id, int) or not _is_member(user.id, chat_id):
                    continue
                others = [uid for uid in _member_ids(chat_id) if uid != user.id]
                await manager.send_to_users(others, {
                    "type": "typing", "chat_id": chat_id, "user_id": user.id, "name": user.full_name,
                })
            # ping/keepalive — просто игнорируем прочее
    except WebSocketDisconnect:
        manager.remove(user.id, websocket)
    except Exception:
        manager.remove(user.id, websocket)
