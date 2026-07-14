"""forum: topics and posts

Revision ID: e7a1c9d84f20
Revises: d6f3b7c04e88
Create Date: 2026-07-14 12:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'e7a1c9d84f20'
down_revision: Union[str, None] = 'd6f3b7c04e88'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'forum_topics',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('title', sa.String(length=255), nullable=False),
        sa.Column('author_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='SET NULL'), nullable=True),
        sa.Column('pinned', sa.Boolean(), nullable=False, server_default='false'),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
    )
    op.create_index('ix_forum_topics_pinned', 'forum_topics', ['pinned'])
    op.create_table(
        'forum_posts',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('topic_id', sa.Integer(), sa.ForeignKey('forum_topics.id', ondelete='CASCADE'), nullable=False),
        sa.Column('author_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='SET NULL'), nullable=True),
        sa.Column('body', sa.Text(), nullable=False),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.Column('edited_at', sa.DateTime(timezone=True), nullable=True),
        sa.Column('edit_count', sa.Integer(), nullable=False, server_default='0'),
    )
    op.create_index('ix_forum_posts_topic_id', 'forum_posts', ['topic_id'])

    # добавить права форума существующим системным ролям (гуру — superadmin, права уже все)
    roles = sa.table('roles', sa.column('key', sa.String), sa.column('capabilities', sa.JSON))
    conn = op.get_bind()
    add_caps = {
        'student': ['forum.view', 'forum.post'],
        'curator': ['forum.view', 'forum.post'],
        'secretary': ['forum.view', 'forum.post', 'forum.moderate'],
    }
    for key, extra in add_caps.items():
        row = conn.execute(sa.select(roles.c.capabilities).where(roles.c.key == key)).first()
        if row is not None:
            caps = list(row[0] or [])
            for c in extra:
                if c not in caps:
                    caps.append(c)
            conn.execute(roles.update().where(roles.c.key == key).values(capabilities=caps))


def downgrade() -> None:
    op.drop_index('ix_forum_posts_topic_id', 'forum_posts')
    op.drop_table('forum_posts')
    op.drop_index('ix_forum_topics_pinned', 'forum_topics')
    op.drop_table('forum_topics')
