"""forum post likes

Revision ID: c2e8b6d1a370
Revises: b1d7f3a9c260
Create Date: 2026-07-14 16:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'c2e8b6d1a370'
down_revision: Union[str, None] = 'b1d7f3a9c260'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'forum_post_likes',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('post_id', sa.Integer(), sa.ForeignKey('forum_posts.id', ondelete='CASCADE'), nullable=False),
        sa.Column('user_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='CASCADE'), nullable=False),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.UniqueConstraint('post_id', 'user_id', name='uq_forum_post_like'),
    )
    op.create_index('ix_forum_post_likes_post_id', 'forum_post_likes', ['post_id'])


def downgrade() -> None:
    op.drop_index('ix_forum_post_likes_post_id', 'forum_post_likes')
    op.drop_table('forum_post_likes')
