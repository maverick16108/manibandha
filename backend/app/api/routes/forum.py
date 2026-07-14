from datetime import datetime, timedelta, timezone

from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy import func
from sqlalchemy.orm import Session, joinedload

from app.api.deps import get_current_user, require_cap
from app.core.database import get_db
from app.models import ForumPost, ForumPostLike, ForumSection, ForumTopic, ForumTopicRead, User
from app.schemas.forum import (
    Participant, PostCreate, PostOut, SectionCreate, SectionOut, SectionUpdate, TopicCreate, TopicListItem, TopicOut,
)

router = APIRouter(prefix="/forum", tags=["forum"])

EDIT_WINDOW = timedelta(hours=1)


def _within_edit_window(p: ForumPost) -> bool:
    created = p.created_at
    if created is None:
        return False
    if created.tzinfo is None:
        created = created.replace(tzinfo=timezone.utc)
    return (datetime.now(timezone.utc) - created) <= EDIT_WINDOW


def _post_out(p: ForumPost, user_id: int | None = None) -> PostOut:
    likers = [Participant(name=l.user.full_name if l.user else None,
                          avatar=l.user.avatar_url if l.user else None) for l in (p.likes or [])]
    return PostOut(
        id=p.id, author_id=p.author_id,
        author_name=p.author.full_name if p.author else None,
        author_avatar=p.author.avatar_url if p.author else None,
        body=p.body, created_at=p.created_at, edit_count=p.edit_count or 0,
        likes=len(p.likes or []),
        liked=any(l.user_id == user_id for l in (p.likes or [])),
        likers=likers,
    )


# ── Разделы ──
def _section_out(s: ForumSection, user: User, is_mod: bool, count: int | None = None) -> SectionOut:
    return SectionOut(
        id=s.id, title=s.title, description=s.description, color=s.color, cover_url=s.cover_url,
        author_id=s.author_id, author_name=s.author.full_name if s.author else None,
        topics_count=count if count is not None else len(s.topics),
        can_edit=(s.author_id == user.id or is_mod), created_at=s.created_at,
    )


@router.get("/sections", response_model=list[SectionOut])
def list_sections(db: Session = Depends(get_db), user: User = Depends(require_cap("forum.view"))):
    from app.core.capabilities import has_cap
    is_mod = has_cap(db, user, "forum.moderate")
    sections = (
        db.query(ForumSection).options(joinedload(ForumSection.author), joinedload(ForumSection.topics))
        .order_by(ForumSection.title.asc()).all()
    )
    return [_section_out(s, user, is_mod) for s in sections]


@router.post("/sections", response_model=SectionOut, status_code=status.HTTP_201_CREATED)
def create_section(payload: SectionCreate, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    title = (payload.title or "").strip()
    if not title:
        raise HTTPException(status_code=400, detail="Нужно название раздела")
    color = (payload.color or "#c8742a").strip()[:16] or "#c8742a"
    s = ForumSection(title=title[:160], description=(payload.description or "").strip()[:500] or None,
                     color=color, cover_url=(payload.cover_url or None), author_id=user.id)
    db.add(s)
    db.commit()
    db.refresh(s)
    return _section_out(s, user, is_mod=True, count=0)


def _section_editable(db: Session, user: User, s: ForumSection):
    from app.core.capabilities import has_cap
    if s.author_id != user.id and not has_cap(db, user, "forum.moderate"):
        raise HTTPException(status_code=403, detail="Менять раздел может создатель или модератор")


@router.patch("/sections/{section_id}", response_model=SectionOut)
def update_section(section_id: int, payload: SectionUpdate, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    s = db.get(ForumSection, section_id)
    if not s:
        raise HTTPException(status_code=404, detail="Раздел не найден")
    _section_editable(db, user, s)
    if payload.title is not None:
        t = payload.title.strip()
        if not t:
            raise HTTPException(status_code=400, detail="Название не может быть пустым")
        s.title = t[:160]
    if payload.description is not None:
        s.description = payload.description.strip()[:500] or None
    if payload.color is not None:
        s.color = (payload.color.strip()[:16] or s.color)
    if payload.cover_url is not None:
        s.cover_url = payload.cover_url or None
    db.commit()
    db.refresh(s)
    from app.core.capabilities import has_cap
    return _section_out(s, user, is_mod=has_cap(db, user, "forum.moderate"))


@router.delete("/sections/{section_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_section(section_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    s = db.get(ForumSection, section_id)
    if not s:
        raise HTTPException(status_code=404, detail="Раздел не найден")
    _section_editable(db, user, s)
    db.delete(s)
    db.commit()


# ── Темы ──
def _participants(posts: list[ForumPost], limit: int = 5) -> list[Participant]:
    """Последние участники темы (без повторов), самые недавние первыми."""
    seen: set[int] = set()
    out: list[Participant] = []
    for p in reversed(posts):  # posts отсортированы по created_at asc → идём с конца
        a = p.author
        if not a or a.id in seen:
            continue
        seen.add(a.id)
        out.append(Participant(name=a.full_name, avatar=a.avatar_url))
        if len(out) >= limit:
            break
    return out


@router.get("/topics", response_model=list[TopicListItem])
def list_topics(section_id: int | None = Query(None), db: Session = Depends(get_db),
                user: User = Depends(require_cap("forum.view"))):
    q = db.query(ForumTopic).options(
        joinedload(ForumTopic.posts).joinedload(ForumPost.author),
        joinedload(ForumTopic.section),
        joinedload(ForumTopic.author),
    )
    if section_id:
        q = q.filter(ForumTopic.section_id == section_id)
    topics = q.order_by(ForumTopic.pinned.desc(), ForumTopic.updated_at.desc()).all()
    reads = {r.topic_id: r.last_seen_at for r in db.query(ForumTopicRead).filter(ForumTopicRead.user_id == user.id).all()}
    out = []
    for t in topics:
        pc = len(t.posts)
        ls = reads.get(t.id)
        unread = ls is None or (t.updated_at and ls and t.updated_at > ls)
        out.append(TopicListItem(
            id=t.id, title=t.title, cover_url=t.cover_url, section_id=t.section_id,
            section_title=t.section.title if t.section else None,
            section_color=t.section.color if t.section else "#c8742a",
            author_name=t.author.full_name if t.author else None,
            pinned=t.pinned, replies=max(0, pc - 1), views=t.views or 0, posts_count=pc,
            participants=_participants(t.posts),
            unread=bool(unread), last_activity=t.updated_at, created_at=t.created_at,
        ))
    return out


def _topic_out(t: ForumTopic, user_id: int | None = None) -> TopicOut:
    return TopicOut(
        id=t.id, title=t.title, cover_url=t.cover_url, section_id=t.section_id,
        section_title=t.section.title if t.section else None,
        section_color=t.section.color if t.section else "#c8742a",
        author_name=t.author.full_name if t.author else None,
        pinned=t.pinned, created_at=t.created_at,
        posts=[_post_out(p, user_id) for p in t.posts],
    )


@router.post("/topics", response_model=TopicOut, status_code=status.HTTP_201_CREATED)
def create_topic(payload: TopicCreate, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    title = (payload.title or "").strip()
    body = (payload.body or "").strip()
    if not title:
        raise HTTPException(status_code=400, detail="Нужен заголовок темы")
    if not body:
        raise HTTPException(status_code=400, detail="Нужно первое сообщение")
    if not db.get(ForumSection, payload.section_id):
        raise HTTPException(status_code=400, detail="Выберите раздел")
    topic = ForumTopic(section_id=payload.section_id, title=title[:255], author_id=user.id,
                       cover_url=(payload.cover_url or None))
    db.add(topic)
    db.flush()
    db.add(ForumPost(topic_id=topic.id, author_id=user.id, body=body))
    db.commit()
    db.refresh(topic)
    return _topic_out(topic, user.id)


@router.get("/topics/{topic_id}", response_model=TopicOut)
def get_topic(topic_id: int, count: bool = Query(True), db: Session = Depends(get_db), user: User = Depends(require_cap("forum.view"))):
    t = db.query(ForumTopic).options(
        joinedload(ForumTopic.posts).joinedload(ForumPost.author),
        joinedload(ForumTopic.posts).joinedload(ForumPost.likes).joinedload(ForumPostLike.user),
        joinedload(ForumTopic.section),
        joinedload(ForumTopic.author),
    ).filter(ForumTopic.id == topic_id).first()
    if not t:
        raise HTTPException(status_code=404, detail="Тема не найдена")
    if count:
        t.views = (t.views or 0) + 1  # только реальное открытие, не фоновый опрос
    # отметить прочитанной
    r = db.query(ForumTopicRead).filter(ForumTopicRead.topic_id == topic_id, ForumTopicRead.user_id == user.id).first()
    if r:
        r.last_seen_at = func.now()
    else:
        db.add(ForumTopicRead(topic_id=topic_id, user_id=user.id))
    db.commit()
    db.refresh(t)
    return _topic_out(t, user.id)


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
    # автор только что видел тему
    r = db.query(ForumTopicRead).filter(ForumTopicRead.topic_id == topic_id, ForumTopicRead.user_id == user.id).first()
    if r:
        r.last_seen_at = func.now()
    else:
        db.add(ForumTopicRead(topic_id=topic_id, user_id=user.id))
    db.commit()
    return _post_out(post, user.id)


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
    return _post_out(post, user.id)


@router.post("/posts/{post_id}/like")
def toggle_like(post_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.view"))):
    post = db.get(ForumPost, post_id)
    if not post:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    existing = db.query(ForumPostLike).filter(ForumPostLike.post_id == post_id, ForumPostLike.user_id == user.id).first()
    if existing:
        db.delete(existing)
    else:
        db.add(ForumPostLike(post_id=post_id, user_id=user.id))
    db.commit()
    likes = (
        db.query(ForumPostLike).options(joinedload(ForumPostLike.user))
        .filter(ForumPostLike.post_id == post_id).all()
    )
    return {
        "likes": len(likes),
        "liked": any(l.user_id == user.id for l in likes),
        "likers": [{"name": l.user.full_name if l.user else None, "avatar": l.user.avatar_url if l.user else None} for l in likes],
    }


@router.delete("/posts/{post_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_post(post_id: int, db: Session = Depends(get_db), user: User = Depends(require_cap("forum.post"))):
    post = db.get(ForumPost, post_id)
    if not post:
        raise HTTPException(status_code=404, detail="Сообщение не найдено")
    _own_or_moderator(db, user, post, need_window=True)
    topic_id = post.topic_id
    db.delete(post)
    db.flush()
    if db.query(ForumPost).filter(ForumPost.topic_id == topic_id).count() == 0:
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
