"""conference bans

Revision ID: d1a7b3e50c92
Revises: c9e4a1f72d05
Create Date: 2026-07-15 11:30:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'd1a7b3e50c92'
down_revision: Union[str, None] = 'c9e4a1f72d05'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'conference_bans',
        sa.Column('id', sa.Integer(), nullable=False),
        sa.Column('conference_id', sa.Integer(), nullable=False),
        sa.Column('identity', sa.String(length=80), nullable=False),
        sa.Column('name', sa.String(length=120), nullable=True),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=True),
        sa.ForeignKeyConstraint(['conference_id'], ['conferences.id'], ondelete='CASCADE'),
        sa.PrimaryKeyConstraint('id'),
        sa.UniqueConstraint('conference_id', 'identity', name='uq_conf_ban'),
    )
    op.create_index('ix_conference_bans_conference_id', 'conference_bans', ['conference_id'])


def downgrade() -> None:
    op.drop_index('ix_conference_bans_conference_id', table_name='conference_bans')
    op.drop_table('conference_bans')
