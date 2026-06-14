package service

import (
	"context"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
)

type RoleService interface {
	ListRoles(ctx context.Context) ([]*domain.Role, error)
	ListPermissions(ctx context.Context) ([]*domain.Permission, error)
	CreateRole(ctx context.Context, role *domain.Role) error
	UpdateRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	LoadRolePermissions(ctx context.Context) (map[string][]string, error)
}

type roleService struct {
	repo repository.RoleRepository
}

func NewRoleService(repo repository.RoleRepository) RoleService {
	return &roleService{
		repo: repo,
	}
}

func (s *roleService) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	return s.repo.ListRoles(ctx)
}

func (s *roleService) ListPermissions(ctx context.Context) ([]*domain.Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *roleService) CreateRole(ctx context.Context, role *domain.Role) error {
	role.ID = uuid.New()
	return s.repo.CreateRole(ctx, role)
}

func (s *roleService) UpdateRolePermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	return s.repo.UpdateRolePermissions(ctx, roleID, permissionIDs)
}

func (s *roleService) LoadRolePermissions(ctx context.Context) (map[string][]string, error) {
	return s.repo.LoadRolePermissions(ctx)
}
