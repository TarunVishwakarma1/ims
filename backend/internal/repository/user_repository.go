package repository

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error)
	WithTx(tx pgx.Tx) UserRepository
}

type userRepository struct {
	db DBTX
}

func NewUserRepository(db DBTX) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) WithTx(tx pgx.Tx) UserRepository {
	return &userRepository{db: tx}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, org_id, name, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query, user.ID, user.OrgID, user.Name, user.Email, user.PasswordHash, user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, org_id, name, email, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE id = $1 AND org_id = $2
	`
	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, id, orgID).Scan(&user.ID, &user.OrgID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, org_id, name, email, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, email).Scan(&user.ID, &user.OrgID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET name = $3, email = $4, password_hash = $5, role = $6, is_active = $7, updated_at = $8
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, user.ID, user.OrgID, user.Name, user.Email, user.PasswordHash, user.Role, user.IsActive, user.UpdatedAt)
	return err
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	query := `
		UPDATE users
		SET is_active = false, updated_at = NOW()
		WHERE id = $1 AND org_id = $2
	`
	_, err := r.db.Exec(ctx, query, id, orgID)
	return err
}

func (r *userRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
	query := `
		SELECT id, org_id, name, email, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE org_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*domain.User, 0)
	for rows.Next() {
		user := &domain.User{}
		err := rows.Scan(&user.ID, &user.OrgID, &user.Name, &user.Email, &user.PasswordHash, &user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
