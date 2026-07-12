"""thread staff_seen_at (shared unread marker)

Revision ID: a3c9e5f7b024
Revises: f2a4b6c8d013
Create Date: 2026-07-12 02:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'a3c9e5f7b024'
down_revision: Union[str, None] = 'f2a4b6c8d013'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('threads', sa.Column('staff_seen_at', sa.DateTime(timezone=True), nullable=True))


def downgrade() -> None:
    op.drop_column('threads', 'staff_seen_at')
