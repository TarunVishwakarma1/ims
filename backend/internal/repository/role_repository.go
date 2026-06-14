package repository

import (
	"context"
	"encoding/json"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleRepository interface {
	ListRoles(ctx context.Context) ([]*domain.Role, error)
	ListPermissions(ctx context.Context) ([]*domain.Permission, error)
	CreateRole(ctx context.Context, role *domain.Role) error
	UpdateRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	LoadRolePermissions(ctx context.Context) (map[string][]string, error)
}

type roleRepository struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) RoleRepository {
	return &roleRepository{db: db}
}

func (r *roleRepository) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	query := `
		SELECT r.id, r.name, r.description, r.is_system, r.created_at, r.updated_at,
		       COALESCE(
		           (SELECT json_agg(json_build_object(
		               'id', p.id,
		               'resource', p.resource,
		               'action', p.action,
		               'description', p.description
		           ))
		            FROM role_permissions rp
		            JOIN permissions p ON p.id = rp.permission_id
		            WHERE rp.role_id = r.id),
		           '[]'::json
		       ) as permissions_json
		FROM roles r
		ORDER BY r.name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]*domain.Role, 0)
	for rows.Next() {
		var role domain.Role
		var permissionsJSON []byte

		err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.IsSystem,
			&role.CreatedAt,
			&role.UpdatedAt,
			&permissionsJSON,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal the JSON array of permissions into the role struct
		if err := json.Unmarshal(permissionsJSON, &role.Permissions); err != nil {
			return nil, err
		}

		roles = append(roles, &role)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

func (r *roleRepository) ListPermissions(ctx context.Context) ([]*domain.Permission, error) {
	query := `SELECT id, resource, action, description FROM permissions ORDER BY resource, action`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]*domain.Permission, 0)
	for rows.Next() {
		var p domain.Permission
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description); err != nil {
			return nil, err
		}
		permissions = append(permissions, &p)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (r *roleRepository) CreateRole(ctx context.Context, role *domain.Role) error {
	query := `
		INSERT INTO roles (id, name, description, is_system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, role.ID, role.Name, role.Description, role.IsSystem, role.CreatedAt, role.UpdatedAt)
	return err
}

func (r *roleRepository) UpdateRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete existing permissions for this role
	_, err = tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return err
	}

	// Insert new permissions
	if len(permissionIDs) > 0 {
		insertQuery := `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`
		for _, permID := range permissionIDs {
			_, err = tx.Exec(ctx, insertQuery, roleID, permID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *roleRepository) LoadRolePermissions(ctx context.Context) (map[string][]string, error) {
	query := `
		SELECT ro.name, p.resource || ':' || p.action
		FROM role_permissions rp
		JOIN roles ro ON ro.id = rp.role_id
		JOIN permissions p ON p.id = rp.permission_id
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rolePerms := make(map[string][]string)
	for rows.Next() {
		var roleName, perm string
		if err := rows.Scan(&roleName, &perm); err != nil {
			return nil, err
		}
		rolePerms[roleName] = append(rolePerms[roleName], perm)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return rolePerms, nil
}
