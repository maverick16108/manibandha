"""sms registration: user phone, disciple approval, sms codes

Revision ID: d5a90b3c71e4
Revises: c3f7a1e42d90
Create Date: 2026-07-11 22:10:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'd5a90b3c71e4'
down_revision: Union[str, None] = 'c3f7a1e42d90'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('users', sa.Column('phone', sa.String(length=20), nullable=True))
    op.create_index(op.f('ix_users_phone'), 'users', ['phone'], unique=True)

    op.add_column('disciples', sa.Column('is_approved', sa.Boolean(), server_default=sa.text('true'), nullable=False))
    op.create_index(op.f('ix_disciples_is_approved'), 'disciples', ['is_approved'], unique=False)

    op.create_table(
        'sms_codes',
        sa.Column('id', sa.Integer(), nullable=False),
        sa.Column('phone', sa.String(length=20), nullable=False),
        sa.Column('code', sa.String(length=8), nullable=False),
        sa.Column('expires_at', sa.DateTime(timezone=True), nullable=False),
        sa.Column('attempts', sa.Integer(), server_default=sa.text('0'), nullable=False),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
        sa.PrimaryKeyConstraint('id'),
    )
    op.create_index(op.f('ix_sms_codes_phone'), 'sms_codes', ['phone'], unique=False)


def downgrade() -> None:
    op.drop_index(op.f('ix_sms_codes_phone'), table_name='sms_codes')
    op.drop_table('sms_codes')
    op.drop_index(op.f('ix_disciples_is_approved'), table_name='disciples')
    op.drop_column('disciples', 'is_approved')
    op.drop_index(op.f('ix_users_phone'), table_name='users')
    op.drop_column('users', 'phone')
