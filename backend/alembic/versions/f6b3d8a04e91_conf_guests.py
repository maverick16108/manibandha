"""conference guests_allowed

Revision ID: f6b3d8a04e91
Revises: e4a2c7f91d63
Create Date: 2026-07-14 21:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'f6b3d8a04e91'
down_revision: Union[str, None] = 'e4a2c7f91d63'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('conferences', sa.Column('guests_allowed', sa.Boolean(), nullable=False, server_default='false'))


def downgrade() -> None:
    op.drop_column('conferences', 'guests_allowed')
