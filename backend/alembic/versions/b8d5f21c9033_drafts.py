"""drafts table

Revision ID: b8d5f21c9033
Revises: a7c4d9e10b22
Create Date: 2026-07-11 20:05:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'b8d5f21c9033'
down_revision: Union[str, None] = 'a7c4d9e10b22'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'drafts',
        sa.Column('id', sa.Integer(), nullable=False),
        sa.Column('user_id', sa.Integer(), nullable=False),
        sa.Column('scope', sa.String(length=64), nullable=False),
        sa.Column('body', sa.Text(), nullable=False),
        sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ondelete='CASCADE'),
        sa.PrimaryKeyConstraint('id'),
        sa.UniqueConstraint('user_id', 'scope', name='uq_draft_user_scope'),
    )
    op.create_index(op.f('ix_drafts_user_id'), 'drafts', ['user_id'], unique=False)


def downgrade() -> None:
    op.drop_index(op.f('ix_drafts_user_id'), table_name='drafts')
    op.drop_table('drafts')
