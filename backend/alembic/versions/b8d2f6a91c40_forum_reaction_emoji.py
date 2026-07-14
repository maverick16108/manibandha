"""forum post like emoji (reactions)

Revision ID: b8d2f6a91c40
Revises: a7c1e5f30b82
Create Date: 2026-07-14 23:30:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'b8d2f6a91c40'
down_revision: Union[str, None] = 'a7c1e5f30b82'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('forum_post_likes', sa.Column('emoji', sa.String(length=16), nullable=False, server_default='❤️'))


def downgrade() -> None:
    op.drop_column('forum_post_likes', 'emoji')
