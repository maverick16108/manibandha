"""dynamic roles and capabilities

Revision ID: c3f7a1e42d90
Revises: b8d5f21c9033
Create Date: 2026-07-11 21:30:00.000000

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'c3f7a1e42d90'
down_revision: Union[str, None] = 'b8d5f21c9033'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.create_table(
        'roles',
        sa.Column('id', sa.Integer(), nullable=False),
        sa.Column('key', sa.String(length=64), nullable=False),
        sa.Column('name', sa.String(length=120), nullable=False),
        sa.Column('is_system', sa.Boolean(), server_default=sa.text('false'), nullable=False),
        sa.Column('is_superadmin', sa.Boolean(), server_default=sa.text('false'), nullable=False),
        sa.Column('is_default', sa.Boolean(), server_default=sa.text('false'), nullable=False),
        sa.Column('capabilities', sa.JSON(), server_default=sa.text("'[]'::json"), nullable=False),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
        sa.PrimaryKeyConstraint('id'),
        sa.UniqueConstraint('key', name='uq_roles_key'),
    )
    op.create_table(
        'user_roles',
        sa.Column('id', sa.Integer(), nullable=False),
        sa.Column('user_id', sa.Integer(), nullable=False),
        sa.Column('role_id', sa.Integer(), nullable=False),
        sa.ForeignKeyConstraint(['user_id'], ['users.id'], ondelete='CASCADE'),
        sa.ForeignKeyConstraint(['role_id'], ['roles.id'], ondelete='CASCADE'),
        sa.PrimaryKeyConstraint('id'),
        sa.UniqueConstraint('user_id', 'role_id', name='uq_user_role'),
    )
    op.create_index(op.f('ix_user_roles_user_id'), 'user_roles', ['user_id'], unique=False)
    op.create_index(op.f('ix_user_roles_role_id'), 'user_roles', ['role_id'], unique=False)

    # seed system roles
    from app.core.capabilities import SYSTEM_ROLES
    roles_table = sa.table(
        'roles',
        sa.column('key', sa.String), sa.column('name', sa.String),
        sa.column('is_system', sa.Boolean), sa.column('is_superadmin', sa.Boolean),
        sa.column('is_default', sa.Boolean), sa.column('capabilities', sa.JSON),
    )
    op.bulk_insert(roles_table, [
        {"key": s["key"], "name": s["name"], "is_system": True,
         "is_superadmin": s["is_superadmin"], "is_default": s["is_default"],
         "capabilities": list(s["capabilities"])}
        for s in SYSTEM_ROLES
    ])

    # assign each existing user the role matching their legacy enum value
    op.execute(sa.text(
        "INSERT INTO user_roles (user_id, role_id) "
        "SELECT u.id, r.id FROM users u JOIN roles r ON r.key = u.role::text"
    ))

    # drop the old section-based permissions table
    op.execute("DROP TABLE IF EXISTS role_permissions")


def downgrade() -> None:
    op.drop_index(op.f('ix_user_roles_role_id'), table_name='user_roles')
    op.drop_index(op.f('ix_user_roles_user_id'), table_name='user_roles')
    op.drop_table('user_roles')
    op.drop_table('roles')
