"""chat messenger: chats, members, messages + global seq

Revision ID: d8f2a3c19b70
Revises: c7e1a2b34d05
Create Date: 2026-07-16 00:30:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'd8f2a3c19b70'
down_revision: Union[str, None] = 'c7e1a2b34d05'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # глобальная монотонная последовательность порядка сообщений (для sync-курсора pts)
    op.execute("CREATE SEQUENCE IF NOT EXISTS chat_message_seq")

    op.create_table(
        'chats',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('type', sa.String(length=20), nullable=False),
        sa.Column('title', sa.String(length=255), nullable=True),
        sa.Column('photo_url', sa.String(length=512), nullable=True),
        sa.Column('created_by', sa.Integer(), sa.ForeignKey('users.id', ondelete='SET NULL'), nullable=True),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
    )
    op.create_index('ix_chats_type', 'chats', ['type'])

    op.create_table(
        'chat_members',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('chat_id', sa.Integer(), sa.ForeignKey('chats.id', ondelete='CASCADE'), nullable=False),
        sa.Column('user_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='CASCADE'), nullable=False),
        sa.Column('role', sa.String(length=16), server_default='member', nullable=False),
        sa.Column('last_read_seq', sa.BigInteger(), server_default='0', nullable=False),
        sa.Column('joined_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.UniqueConstraint('chat_id', 'user_id', name='uq_chat_member'),
    )
    op.create_index('ix_chat_members_chat_id', 'chat_members', ['chat_id'])
    op.create_index('ix_chat_members_user_id', 'chat_members', ['user_id'])

    op.create_table(
        'chat_messages',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('chat_id', sa.Integer(), sa.ForeignKey('chats.id', ondelete='CASCADE'), nullable=False),
        sa.Column('seq', sa.BigInteger(), nullable=False),
        sa.Column('client_uuid', sa.String(length=64), nullable=True),
        sa.Column('author_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='SET NULL'), nullable=True),
        sa.Column('body', sa.Text(), nullable=False),
        sa.Column('reply_to_id', sa.Integer(), sa.ForeignKey('chat_messages.id', ondelete='SET NULL'), nullable=True),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.Column('edited_at', sa.DateTime(timezone=True), nullable=True),
        sa.Column('edit_count', sa.Integer(), server_default='0', nullable=False),
        sa.Column('deleted', sa.Boolean(), server_default='false', nullable=False),
        sa.UniqueConstraint('chat_id', 'client_uuid', name='uq_chat_message_uuid'),
        sa.UniqueConstraint('seq', name='uq_chat_message_seq_val'),
    )
    op.create_index('ix_chat_messages_chat_id', 'chat_messages', ['chat_id'])
    op.create_index('ix_chat_messages_seq', 'chat_messages', ['seq'])


def downgrade() -> None:
    op.drop_table('chat_messages')
    op.drop_table('chat_members')
    op.drop_index('ix_chats_type', table_name='chats')
    op.drop_table('chats')
    op.execute("DROP SEQUENCE IF EXISTS chat_message_seq")
