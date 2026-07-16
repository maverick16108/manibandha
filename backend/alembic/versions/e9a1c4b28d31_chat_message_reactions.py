"""chat message reactions

Revision ID: e9a1c4b28d31
Revises: d8f2a3c19b70
Create Date: 2026-07-16 01:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'e9a1c4b28d31'
down_revision: Union[str, None] = 'd8f2a3c19b70'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'chat_message_reactions',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('message_id', sa.Integer(), sa.ForeignKey('chat_messages.id', ondelete='CASCADE'), nullable=False),
        sa.Column('user_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='CASCADE'), nullable=False),
        sa.Column('emoji', sa.String(length=16), nullable=False),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.UniqueConstraint('message_id', 'user_id', name='uq_chat_reaction'),
    )
    op.create_index('ix_chat_message_reactions_message_id', 'chat_message_reactions', ['message_id'])


def downgrade() -> None:
    op.drop_table('chat_message_reactions')
