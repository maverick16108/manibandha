"""forum sections, views, topic reads

Revision ID: a9c4e2f70b18
Revises: f8b2d5e91a44
Create Date: 2026-07-14 14:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'a9c4e2f70b18'
down_revision: Union[str, None] = 'f8b2d5e91a44'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'forum_sections',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('title', sa.String(length=160), nullable=False),
        sa.Column('description', sa.String(length=500), nullable=True),
        sa.Column('color', sa.String(length=16), nullable=False, server_default='#c8742a'),
        sa.Column('author_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='SET NULL'), nullable=True),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
    )
    op.add_column('forum_topics', sa.Column('section_id', sa.Integer(), sa.ForeignKey('forum_sections.id', ondelete='CASCADE'), nullable=True))
    op.create_index('ix_forum_topics_section_id', 'forum_topics', ['section_id'])
    op.add_column('forum_topics', sa.Column('views', sa.Integer(), nullable=False, server_default='0'))

    op.create_table(
        'forum_topic_reads',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('topic_id', sa.Integer(), sa.ForeignKey('forum_topics.id', ondelete='CASCADE'), nullable=False),
        sa.Column('user_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='CASCADE'), nullable=False),
        sa.Column('last_seen_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.UniqueConstraint('topic_id', 'user_id', name='uq_forum_topic_read'),
    )
    op.create_index('ix_forum_topic_reads_topic_id', 'forum_topic_reads', ['topic_id'])
    op.create_index('ix_forum_topic_reads_user_id', 'forum_topic_reads', ['user_id'])

    # раздел по умолчанию для уже существующих тем
    conn = op.get_bind()
    has_topics = conn.execute(sa.text("SELECT 1 FROM forum_topics LIMIT 1")).first()
    if has_topics:
        conn.execute(sa.text(
            "INSERT INTO forum_sections (title, description, color) VALUES ('Общее', 'Общие темы', '#c8742a')"
        ))
        sid = conn.execute(sa.text("SELECT id FROM forum_sections ORDER BY id DESC LIMIT 1")).scalar()
        conn.execute(sa.text("UPDATE forum_topics SET section_id = :sid WHERE section_id IS NULL"), {"sid": sid})


def downgrade() -> None:
    op.drop_index('ix_forum_topic_reads_user_id', 'forum_topic_reads')
    op.drop_index('ix_forum_topic_reads_topic_id', 'forum_topic_reads')
    op.drop_table('forum_topic_reads')
    op.drop_column('forum_topics', 'views')
    op.drop_index('ix_forum_topics_section_id', 'forum_topics')
    op.drop_column('forum_topics', 'section_id')
    op.drop_table('forum_sections')
