"""disciple mentor_name (personal mentor free text)

Revision ID: f2a4b6c8d013
Revises: e1b2c3d4f506
Create Date: 2026-07-12 01:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'f2a4b6c8d013'
down_revision: Union[str, None] = 'e1b2c3d4f506'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('disciples', sa.Column('mentor_name', sa.String(length=255), nullable=True))


def downgrade() -> None:
    op.drop_column('disciples', 'mentor_name')
