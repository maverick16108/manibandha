"""recording title + description

Revision ID: f3b9d21c4a80
Revises: e2f5c8a41b73
Create Date: 2026-07-15 21:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'f3b9d21c4a80'
down_revision: Union[str, None] = 'e2f5c8a41b73'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('conference_recordings', sa.Column('title', sa.String(length=255), nullable=True))
    op.add_column('conference_recordings', sa.Column('description', sa.Text(), nullable=True))


def downgrade() -> None:
    op.drop_column('conference_recordings', 'description')
    op.drop_column('conference_recordings', 'title')
