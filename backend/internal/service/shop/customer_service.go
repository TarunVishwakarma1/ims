package shop

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
)

// CustomerService handles customer profile and address operations.
type CustomerService interface {
	Get(ctx context.Context, customerID uuid.UUID) (*domain.Customer, error)
	Update(ctx context.Context, customerID uuid.UUID, name, email string) error
	AddAddress(ctx context.Context, customerID uuid.UUID, a *domain.CustomerAddress) (uuid.UUID, error)
	ListAddresses(ctx context.Context, customerID uuid.UUID) ([]domain.CustomerAddress, error)
	UpdateAddress(ctx context.Context, customerID uuid.UUID, a *domain.CustomerAddress) error
	DeleteAddress(ctx context.Context, customerID, addrID uuid.UUID) error
	SetDefaultAddress(ctx context.Context, customerID, addrID uuid.UUID) error
}

type customerService struct {
	custRepo repository.CustomerRepository
	addrRepo repository.CustomerAddressRepository
}

// NewCustomerService returns a CustomerService wiring the given repositories.
func NewCustomerService(c repository.CustomerRepository, a repository.CustomerAddressRepository) CustomerService {
	return &customerService{custRepo: c, addrRepo: a}
}

func (s *customerService) Get(ctx context.Context, customerID uuid.UUID) (*domain.Customer, error) {
	return s.custRepo.GetByID(ctx, customerID)
}

func (s *customerService) Update(ctx context.Context, customerID uuid.UUID, name, email string) error {
	return s.custRepo.UpdateProfile(ctx, customerID, name, email)
}

func (s *customerService) AddAddress(ctx context.Context, customerID uuid.UUID, a *domain.CustomerAddress) (uuid.UUID, error) {
	a.CustomerID = customerID
	return s.addrRepo.Create(ctx, a)
}

func (s *customerService) ListAddresses(ctx context.Context, customerID uuid.UUID) ([]domain.CustomerAddress, error) {
	return s.addrRepo.ListByCustomer(ctx, customerID)
}

// UpdateAddress verifies address ownership before delegating to the repository.
func (s *customerService) UpdateAddress(ctx context.Context, customerID uuid.UUID, a *domain.CustomerAddress) error {
	owned, err := s.addrRepo.GetByID(ctx, a.ID)
	if err != nil {
		return err
	}
	if owned == nil || owned.CustomerID != customerID {
		return errors.New("forbidden")
	}
	a.CustomerID = customerID
	return s.addrRepo.Update(ctx, a)
}

// DeleteAddress verifies ownership then deletes.
func (s *customerService) DeleteAddress(ctx context.Context, customerID, addrID uuid.UUID) error {
	owned, err := s.addrRepo.GetByID(ctx, addrID)
	if err != nil {
		return err
	}
	if owned == nil || owned.CustomerID != customerID {
		return errors.New("forbidden")
	}
	return s.addrRepo.Delete(ctx, addrID, customerID)
}

// SetDefaultAddress verifies ownership then atomically sets the default.
func (s *customerService) SetDefaultAddress(ctx context.Context, customerID, addrID uuid.UUID) error {
	owned, err := s.addrRepo.GetByID(ctx, addrID)
	if err != nil {
		return err
	}
	if owned == nil || owned.CustomerID != customerID {
		return errors.New("forbidden")
	}
	return s.addrRepo.SetDefault(ctx, addrID, customerID)
}
