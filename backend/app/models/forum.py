from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Integer, String, Text, UniqueConstraint, func
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class ForumSection(Base):
    """Раздел форума — категория, внутри которой создаются темы."""

    __tablename__ = "forum_sections"

    id: Mapped[int] = mapped_column(primary_key=True)
    title: Mapped[str] = mapped_column(String(160), nullable=False)
    description: Mapped[str | None] = mapped_column(String(500), nullable=True)
    color: Mapped[str] = mapped_column(String(16), default="#c8742a", server_default="#c8742a", nullable=False)
    cover_url: Mapped[str | None] = mapped_column(String(500), nullable=True)
    author_id: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())

    author = relationship("User")
    topics = relationship("ForumTopic", back_populates="section", cascade="all, delete-orphan")


class ForumTopic(Base):
    """Тема форума для общения учеников."""

    __tablename__ = "forum_topics"

    id: Mapped[int] = mapped_column(primary_key=True)
    section_id: Mapped[int | None] = mapped_column(ForeignKey("forum_sections.id", ondelete="CASCADE"), nullable=True, index=True)
    title: Mapped[str] = mapped_column(String(255), nullable=False)
    author_id: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True)
    pinned: Mapped[bool] = mapped_column(default=False, server_default="false", nullable=False, index=True)
    views: Mapped[int] = mapped_column(Integer, default=0, server_default="0", nullable=False)
    cover_url: Mapped[str | None] = mapped_column(String(500), nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())

    author = relationship("User")
    section = relationship("ForumSection", back_populates="topics")
    posts = relationship(
        "ForumPost", back_populates="topic", cascade="all, delete-orphan", order_by="ForumPost.created_at"
    )


class ForumPost(Base):
    __tablename__ = "forum_posts"

    id: Mapped[int] = mapped_column(primary_key=True)
    topic_id: Mapped[int] = mapped_column(ForeignKey("forum_topics.id", ondelete="CASCADE"), nullable=False, index=True)
    author_id: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"), nullable=True)
    body: Mapped[str] = mapped_column(Text, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    edited_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    edit_count: Mapped[int] = mapped_column(default=0, server_default="0", nullable=False)

    topic = relationship("ForumTopic", back_populates="posts")
    author = relationship("User")


class ForumTopicRead(Base):
    """Когда пользователь в последний раз открывал тему (для индикатора «непрочитано»)."""

    __tablename__ = "forum_topic_reads"
    __table_args__ = (UniqueConstraint("topic_id", "user_id", name="uq_forum_topic_read"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    topic_id: Mapped[int] = mapped_column(ForeignKey("forum_topics.id", ondelete="CASCADE"), nullable=False, index=True)
    user_id: Mapped[int] = mapped_column(ForeignKey("users.id", ondelete="CASCADE"), nullable=False, index=True)
    last_seen_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
