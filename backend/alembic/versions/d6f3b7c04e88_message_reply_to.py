"""thread_messages.reply_to_id (ответ на сообщение)

Revision ID: d6f3b7c04e88
Revises: c5e2a9b31d66
Create Date: 2026-07-12 14:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'd6f3b7c04e88'
down_revision: Union[str, None] = 'c5e2a9b31d66'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('thread_messages', sa.Column('reply_to_id', sa.Integer(), nullable=True))
    op.create_foreign_key(
        'fk_thread_messages_reply_to', 'thread_messages', 'thread_messages',
        ['reply_to_id'], ['id'], ondelete='SET NULL',
    )


def downgrade() -> None:
    op.drop_constraint('fk_thread_messages_reply_to', 'thread_messages', type_='foreignkey')
    op.drop_column('thread_messages', 'reply_to_id')
