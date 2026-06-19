package service

import (
	"context"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type CustomerService interface {
	Register(ctx context.Context, customer *domain.Customer, rawPassword string) error
	Login(ctx context.Context, email, password string) (*domain.Customer, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Customer, error)
	AddAddress(ctx context.Context, address *domain.CustomerAddress) error
	ListAddresses(ctx context.Context, customerID uuid.UUID) ([]*domain.CustomerAddress, error)
}

type customerService struct {
	repo repository.CustomerRepository
}

func NewCustomerService(repo repository.CustomerRepository) CustomerService {
	return &customerService{repo: repo}
}

func (s *customerService) Register(ctx context.Context, customer *domain.Customer, rawPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	customer.PasswordHash = string(hash)
	customer.ID = uuid.New()
	now := time.Now()
	customer.CreatedAt = now
	customer.UpdatedAt = now
	customer.IsVerified = false

	return s.repo.Create(ctx, customer)
}

func (s *customerService) Login(ctx context.Context, email, password string) (*domain.Customer, error) {
	customer, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(customer.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	return customer, nil
}

func (s *customerService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Customer, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *customerService) AddAddress(ctx context.Context, address *domain.CustomerAddress) error {
	address.ID = uuid.New()
	address.CreatedAt = time.Now()
	return s.repo.CreateAddress(ctx, address)
}

func (s *customerService) ListAddresses(ctx context.Context, customerID uuid.UUID) ([]*domain.CustomerAddress, error) {
	return s.repo.ListAddresses(ctx, customerID)
}
