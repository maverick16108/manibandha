"""chat member pinned

Revision ID: a2c4e6f80b13
Revises: f1b3c5d79e42
Create Date: 2026-07-16 03:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'a2c4e6f80b13'
down_revision: Union[str, None] = 'f1b3c5d79e42'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('chat_members', sa.Column('pinned', sa.Boolean(), server_default='false', nullable=False))


def downgrade() -> None:
    op.drop_column('chat_members', 'pinned')
