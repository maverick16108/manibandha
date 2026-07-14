"""disciple notes and files

Revision ID: f8b2d5e91a44
Revises: e7a1c9d84f20
Create Date: 2026-07-14 13:00:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'f8b2d5e91a44'
down_revision: Union[str, None] = 'e7a1c9d84f20'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'disciple_notes',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('disciple_id', sa.Integer(), sa.ForeignKey('disciples.id', ondelete='CASCADE'), nullable=False),
        sa.Column('author_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='SET NULL'), nullable=True),
        sa.Column('text', sa.Text(), nullable=False),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
    )
    op.create_index('ix_disciple_notes_disciple_id', 'disciple_notes', ['disciple_id'])

    op.create_table(
        'disciple_files',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('disciple_id', sa.Integer(), sa.ForeignKey('disciples.id', ondelete='CASCADE'), nullable=False),
        sa.Column('uploaded_by', sa.Integer(), sa.ForeignKey('users.id', ondelete='SET NULL'), nullable=True),
        sa.Column('name', sa.String(length=255), nullable=False),
        sa.Column('url', sa.String(length=500), nullable=False),
        sa.Column('size', sa.Integer(), nullable=True),
        sa.Column('content_type', sa.String(length=120), nullable=True),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
    )
    op.create_index('ix_disciple_files_disciple_id', 'disciple_files', ['disciple_id'])

    # добавить право disciples.note кураторам и секретарю
    roles = sa.table('roles', sa.column('key', sa.String), sa.column('capabilities', sa.JSON))
    conn = op.get_bind()
    for key in ('curator', 'secretary'):
        row = conn.execute(sa.select(roles.c.capabilities).where(roles.c.key == key)).first()
        if row is not None:
            caps = list(row[0] or [])
            if 'disciples.note' not in caps:
                caps.append('disciples.note')
                conn.execute(roles.update().where(roles.c.key == key).values(capabilities=caps))


def downgrade() -> None:
    op.drop_index('ix_disciple_files_disciple_id', 'disciple_files')
    op.drop_table('disciple_files')
    op.drop_index('ix_disciple_notes_disciple_id', 'disciple_notes')
    op.drop_table('disciple_notes')
