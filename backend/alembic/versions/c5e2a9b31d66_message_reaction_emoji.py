"""message reactions: emoji column

Revision ID: c5e2a9b31d66
Revises: b4d1f8a20c55
Create Date: 2026-07-12 13:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'c5e2a9b31d66'
down_revision: Union[str, None] = 'b4d1f8a20c55'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('message_likes', sa.Column('emoji', sa.String(length=16), nullable=False, server_default='❤️'))


def downgrade() -> None:
    op.drop_column('message_likes', 'emoji')
