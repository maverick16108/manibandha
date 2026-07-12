"""thread_messages edited_at + edit_count (edit within 1h)

Revision ID: b4d1f8a20c55
Revises: a3c9e5f7b024
Create Date: 2026-07-12 12:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'b4d1f8a20c55'
down_revision: Union[str, None] = 'a3c9e5f7b024'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('thread_messages', sa.Column('edited_at', sa.DateTime(timezone=True), nullable=True))
    op.add_column('thread_messages', sa.Column('edit_count', sa.Integer(), nullable=False, server_default='0'))


def downgrade() -> None:
    op.drop_column('thread_messages', 'edit_count')
    op.drop_column('thread_messages', 'edited_at')
