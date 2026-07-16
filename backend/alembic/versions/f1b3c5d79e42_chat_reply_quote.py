"""chat message reply_quote

Revision ID: f1b3c5d79e42
Revises: e9a1c4b28d31
Create Date: 2026-07-16 02:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'f1b3c5d79e42'
down_revision: Union[str, None] = 'e9a1c4b28d31'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('chat_messages', sa.Column('reply_quote', sa.Text(), nullable=True))


def downgrade() -> None:
    op.drop_column('chat_messages', 'reply_quote')
