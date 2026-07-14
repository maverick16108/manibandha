"""conferences (video)

Revision ID: d3f9a1b57c48
Revises: c2e8b6d1a370
Create Date: 2026-07-14 18:30:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'd3f9a1b57c48'
down_revision: Union[str, None] = 'c2e8b6d1a370'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'conferences',
        sa.Column('id', sa.Integer(), primary_key=True),
        sa.Column('title', sa.String(length=255), nullable=False),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('mode', sa.String(length=20), nullable=False, server_default='interactive'),
        sa.Column('room', sa.String(length=80), nullable=False),
        sa.Column('status', sa.String(length=20), nullable=False, server_default='scheduled'),
        sa.Column('host_id', sa.Integer(), sa.ForeignKey('users.id', ondelete='SET NULL'), nullable=True),
        sa.Column('scheduled_at', sa.DateTime(timezone=True), nullable=True),
        sa.Column('started_at', sa.DateTime(timezone=True), nullable=True),
        sa.Column('ended_at', sa.DateTime(timezone=True), nullable=True),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.func.now()),
        sa.UniqueConstraint('room', name='uq_conference_room'),
    )
    op.create_index('ix_conferences_status', 'conferences', ['status'])

    # права конференций существующим ролям
    roles = sa.table('roles', sa.column('key', sa.String), sa.column('capabilities', sa.JSON))
    conn = op.get_bind()
    add = {
        'student': ['conference.view'],
        'curator': ['conference.view'],
        'secretary': ['conference.view', 'conference.host'],
        'forum_moderator': ['conference.view'],
    }
    for key, extra in add.items():
        row = conn.execute(sa.select(roles.c.capabilities).where(roles.c.key == key)).first()
        if row is not None:
            caps = list(row[0] or [])
            changed = False
            for c in extra:
                if c not in caps:
                    caps.append(c); changed = True
            if changed:
                conn.execute(roles.update().where(roles.c.key == key).values(capabilities=caps))


def downgrade() -> None:
    op.drop_index('ix_conferences_status', 'conferences')
    op.drop_table('conferences')
