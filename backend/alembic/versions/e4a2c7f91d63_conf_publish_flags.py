"""conference default publish flags

Revision ID: e4a2c7f91d63
Revises: d3f9a1b57c48
Create Date: 2026-07-14 20:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'e4a2c7f91d63'
down_revision: Union[str, None] = 'd3f9a1b57c48'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('conferences', sa.Column('mic_allowed', sa.Boolean(), nullable=False, server_default='true'))
    op.add_column('conferences', sa.Column('cam_allowed', sa.Boolean(), nullable=False, server_default='true'))


def downgrade() -> None:
    op.drop_column('conferences', 'cam_allowed')
    op.drop_column('conferences', 'mic_allowed')
