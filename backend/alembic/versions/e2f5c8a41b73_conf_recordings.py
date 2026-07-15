"""conference recordings + auto_record

Revision ID: e2f5c8a41b73
Revises: d1a7b3e50c92
Create Date: 2026-07-15 20:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'e2f5c8a41b73'
down_revision: Union[str, None] = 'd1a7b3e50c92'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('conferences', sa.Column('auto_record', sa.Boolean(), nullable=False, server_default='false'))
    op.create_table(
        'conference_recordings',
        sa.Column('id', sa.Integer(), nullable=False),
        sa.Column('conference_id', sa.Integer(), nullable=False),
        sa.Column('egress_id', sa.String(length=80), nullable=True),
        sa.Column('filename', sa.String(length=255), nullable=True),
        sa.Column('status', sa.String(length=20), server_default='active', nullable=False),
        sa.Column('duration_ms', sa.BigInteger(), server_default='0', nullable=False),
        sa.Column('size_bytes', sa.BigInteger(), server_default='0', nullable=False),
        sa.Column('started_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=True),
        sa.Column('ended_at', sa.DateTime(timezone=True), nullable=True),
        sa.ForeignKeyConstraint(['conference_id'], ['conferences.id'], ondelete='CASCADE'),
        sa.PrimaryKeyConstraint('id'),
    )
    op.create_index('ix_conference_recordings_conference_id', 'conference_recordings', ['conference_id'])
    op.create_index('ix_conference_recordings_egress_id', 'conference_recordings', ['egress_id'])


def downgrade() -> None:
    op.drop_index('ix_conference_recordings_egress_id', table_name='conference_recordings')
    op.drop_index('ix_conference_recordings_conference_id', table_name='conference_recordings')
    op.drop_table('conference_recordings')
    op.drop_column('conferences', 'auto_record')
