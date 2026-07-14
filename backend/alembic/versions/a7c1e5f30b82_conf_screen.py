"""conference screen_allowed

Revision ID: a7c1e5f30b82
Revises: f6b3d8a04e91
Create Date: 2026-07-14 22:30:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'a7c1e5f30b82'
down_revision: Union[str, None] = 'f6b3d8a04e91'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('conferences', sa.Column('screen_allowed', sa.Boolean(), nullable=False, server_default='true'))


def downgrade() -> None:
    op.drop_column('conferences', 'screen_allowed')
