"""disciple.gender

Revision ID: c7e1a2b34d05
Revises: a4c7e9d10b52
Create Date: 2026-07-16 00:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'c7e1a2b34d05'
down_revision: Union[str, None] = 'a4c7e9d10b52'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('disciples', sa.Column('gender', sa.String(length=20), nullable=True))


def downgrade() -> None:
    op.drop_column('disciples', 'gender')
