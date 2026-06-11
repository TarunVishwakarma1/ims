package service

import (
	"context"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type UserService interface {
	Create(ctx context.Context, user *domain.User, ipAddress string) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uuid.UUID, ipAddress string) error
	List(ctx context.Context) ([]*domain.User, error)
}

type userService struct {
	repo         repository.UserRepository
	auditLogRepo repository.AuditLogRepository
}

func NewUserService(repo repository.UserRepository, auditLogRepo repository.AuditLogRepository) UserService {
	return &userService{
		repo:         repo,
		auditLogRepo: auditLogRepo,
	}
}

func (s *userService) Create(ctx context.Context, user *domain.User, ipAddress string) error {
	existing, err := s.repo.GetByEmail(ctx, user.Email)
	if err == nil && existing != nil {
		return domain.ErrConflict
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	user.ID = uuid.New()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hashedPassword)

	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	if err := s.repo.Create(ctx, user); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		UserID:    &user.ID,
		Action:    "user.created",
		Entity:    "users",
		EntityID:  user.ID,
		IPAddress: ipAddress,
		CreatedAt: now,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	return nil
}

func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *userService) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *userService) Update(ctx context.Context, user *domain.User) error {
	user.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, user)
}

func (s *userService) Delete(ctx context.Context, id uuid.UUID, ipAddress string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		UserID:    nil,
		Action:    "user.deleted",
		Entity:    "users",
		EntityID:  id,
		IPAddress: ipAddress,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	return nil
}

func (s *userService) List(ctx context.Context) ([]*domain.User, error) {
	return s.repo.List(ctx)
}

