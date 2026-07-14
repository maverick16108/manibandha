"""forum topic/section covers + forum moderator role

Revision ID: b1d7f3a9c260
Revises: a9c4e2f70b18
Create Date: 2026-07-14 15:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'b1d7f3a9c260'
down_revision: Union[str, None] = 'a9c4e2f70b18'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('forum_topics', sa.Column('cover_url', sa.String(length=500), nullable=True))
    op.add_column('forum_sections', sa.Column('cover_url', sa.String(length=500), nullable=True))

    # роль «Модератор форума», если ещё нет
    roles = sa.table(
        'roles',
        sa.column('key', sa.String), sa.column('name', sa.String),
        sa.column('is_system', sa.Boolean), sa.column('is_superadmin', sa.Boolean),
        sa.column('is_default', sa.Boolean), sa.column('capabilities', sa.JSON),
    )
    conn = op.get_bind()
    exists = conn.execute(sa.select(roles.c.key).where(roles.c.key == 'forum_moderator')).first()
    if not exists:
        conn.execute(roles.insert().values(
            key='forum_moderator', name='Модератор форума', is_system=True,
            is_superadmin=False, is_default=False,
            capabilities=['forum.view', 'forum.post', 'forum.moderate'],
        ))


def downgrade() -> None:
    op.drop_column('forum_sections', 'cover_url')
    op.drop_column('forum_topics', 'cover_url')
