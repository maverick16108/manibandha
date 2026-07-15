"""conference short code

Revision ID: a4c7e9d10b52
Revises: f3b9d21c4a80
Create Date: 2026-07-16 09:00:00.000000

"""
import secrets
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


revision: str = 'a4c7e9d10b52'
down_revision: Union[str, None] = 'f3b9d21c4a80'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None

_ALPHABET = "abcdefghijkmnpqrstuvwxyzACDEFGHJKLMNPQRSTUVWXYZ23456789"


def _code():
    return "".join(secrets.choice(_ALPHABET) for _ in range(7))


def upgrade() -> None:
    op.add_column('conferences', sa.Column('code', sa.String(length=16), nullable=True))
    conn = op.get_bind()
    rows = conn.execute(sa.text("SELECT id FROM conferences")).fetchall()
    used = set()
    for (cid,) in rows:
        c = _code()
        while c in used:
            c = _code()
        used.add(c)
        conn.execute(sa.text("UPDATE conferences SET code=:c WHERE id=:i"), {"c": c, "i": cid})
    op.create_index('ix_conferences_code', 'conferences', ['code'], unique=True)


def downgrade() -> None:
    op.drop_index('ix_conferences_code', table_name='conferences')
    op.drop_column('conferences', 'code')
