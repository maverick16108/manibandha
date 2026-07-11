"""city region

Revision ID: e1b2c3d4f506
Revises: d5a90b3c71e4
Create Date: 2026-07-12 00:20:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'e1b2c3d4f506'
down_revision: Union[str, None] = 'd5a90b3c71e4'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('cities', sa.Column('region', sa.String(length=160), nullable=True))


def downgrade() -> None:
    op.drop_column('cities', 'region')
