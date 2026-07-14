from datetime import datetime, timedelta, timezone

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy import func
from sqlalchemy.orm import Session, joinedload

from app.api.deps import get_current_user, require_cap
from app.core.database import get_db
from app.models import ForumPost, ForumTopic, User
from app.schemas.forum import PostCreate, PostOut, TopicCreate, TopicListItem, TopicOut

router = APIRouter(prefix="/forum", tags=["forum"])

EDIT_WINDOW = timedelta(hours=1)


def _snippet(body: str) -> str:
    import re
    s = body or ""
    s = re.sub(r"@\[audio\]\([^)]*\)", "🎤 Голосовое", s)
    s = re.sub(r"!\[[^\]]*\]\([^)]*\)", "🖼 Фото", s)
    s = re.sub(r"\s+", " ", s).strip()
    return s[:140]


def _within_edit_window(p: ForumPost) -> bool:
    created = p.created_at
    if created is None:
        return False
    if created.tzinfo is None:
        created = created.replace(tzinfo=timezone.utc)
    return (datetime.now(timezone.utc) - created) <= EDIT_WINDOW


def _post_out(p: ForumPost) -> PostOut:
    return PostOut(
        id=p.id, author_id=p.author_id,
        author_name=p.author.full_name if p.author else None,
        author_avatar=p.author.avatar_url if p.author else None,
        body=p.body, created_at=p.created_at, edit_count=p.edit_count or 0,
    )


@router.get("/topics", response_model=list[TopicListItem])
def list_topics(db: Session = Depends(get_db), _: User = Depends(require_cap("forum.view"))):
    topics = (
        db.query(ForumTopic)
        .options(joinedload(ForumTopic.posts), joinedload(ForumTopic.author))
        .order_by(ForumTopic.pinned.desc(), ForumTopic.updated_at.desc())
        .all()
    )
    out = []
    for t in topics:
        last = t.posts[-1] if t.posts else None
        out.append(TopicListItem(
            id=t.id, title=t.title,
            author_name=t.author.full_name if t.author else None,
            pinned=t.pinned, created_at=t.created_at, updated_at=t.updated_at,
            posts_count=len(t.posts),
            last_post_preview=_snippet(last.body) if last else None,
            last_post_author=(last.author.full_name if last and last.author else None),
            last_post_at=last.created_at if last else None,
        ))
    return out


@router.post("/topics", response_model=TopicOut, status_code=status.HTTP_201_CREATED)
def create_topic(payload: TopicCreate, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    title = (payload.title or "").strip()
    body = (payload.body or "").strip()
    if not title:
        raise HTTPException(status_code=400, detail="Нужен заголовок темы")
    if not body:
        raise HTTPException(status_code=400, detail="Нужно первое сообщение")
    topic = ForumTopic(title=title[:255], author_id=user.id)
    db.add(topic)
    db.flush()
    db.add(ForumPost(topic_id=topic.id, author_id=user.id, body=body))
    db.commit()
    db.refresh(topic)
    return _topic_out(topic)


def _topic_out(t: ForumTopic) -> TopicOut:
    return TopicOut(
        id=t.id, title=t.title,
        author_name=t.author.full_name if t.author else None,
        pinned=t.pinned, created_at=t.created_at,
        posts=[_post_out(p) for p in t.posts],
    )


@router.get("/topics/{topic_id}", response_model=TopicOut)
def get_topic(topic_id: int, db: Session = Depends(get_db), _: User = Depends(require_cap("forum.view"))):
    t = db.query(ForumTopic).options(
        joinedload(ForumTopic.posts).joinedload(ForumPost.author),
        joinedload(ForumTopic.author),
    ).filter(ForumTopic.id == topic_id).first()
    if not t:
        raise HTTPException(status_code=404, detail="Тема не найдена")
    return _topic_out(t)


@router.post("/topics/{topic_id}/posts", response_model=PostOut, status_code=status.HTTP_201_CREATED)
def add_post(topic_id: int, payload: PostCreate, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    t = db.get(ForumTopic, topic_id)
    if not t:
        raise HTTPException(status_code=404, detail="Тема не найдена")
    body = (payload.body or "").strip()
    if not body:
        raise HTTPException(status_code=400, detail="Пустое сообщение")
    post = ForumPost(topic_id=topic_id, author_id=user.id, body=body)
    db.add(post)
    t.updated_at = func.now()
    db.commit()
    db.refresh(post)
    return _post_out(post)


def _own_or_moderator(db: Session, user: User, post: ForumPost, need_window: bool):
    from app.core.capabilities import has_cap
    is_mod = has_cap(db, user, "forum.moderate")
    if post.author_id != user.id and not is_mod:
        raise HTTPException(status_code=403, detail="Можно менять только свои сообщения")
    if not is_mod and need_window and not _within_edit_window(post):
        raise HTTPException(status_code=403, detail="Прошёл час — сообщение больше нельзя изменить")


@router.patch("/posts/{post_id}", response_model=PostOut)
def edit_post(post_id: int, payload: PostCreate, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    post = db.get(ForumPost, post_id)
    if not post:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    _own_or_moderator(db, user, post, need_window=True)
    body = (payload.body or "").strip()
    if not body:
        raise HTTPException(status_code=400, detail="Пустое сообщение")
    post.body = body
    post.edited_at = func.now()
    post.edit_count = (post.edit_count or 0) + 1
    db.commit()
    db.refresh(post)
    return _post_out(post)


@router.delete("/posts/{post_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_post(post_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    post = db.get(ForumPost, post_id)
    if not post:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    _own_or_moderator(db, user, post, need_window=True)
    topic_id = post.topic_id
    db.delete(post)
    db.flush()
    # если это было последнее сообщение темы — удалить и тему
    remaining = db.query(ForumPost).filter(ForumPost.topic_id == topic_id).count()
    if remaining == 0:
        t = db.get(ForumTopic, topic_id)
        if t:
            db.delete(t)
    db.commit()


@router.delete("/topics/{topic_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_topic(topic_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    from app.core.capabilities import has_cap
    t = db.get(ForumTopic, topic_id)
    if not t:
        raise HTTPException(status_code=404, detail="Тема не найдена")
    if t.author_id != user.id and not has_cap(db, user, "forum.moderate"):
        raise HTTPException(status_code=403, detail="Удалять тему может автор или модератор")
    db.delete(t)
    db.commit()
