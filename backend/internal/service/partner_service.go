package service

import (
	"context"
	"errors"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
)

type PartnerService interface {
	// List returns counterparties for the caller. role ∈ "supplier" | "buyer".
	List(ctx context.Context, callerOrgID uuid.UUID, role string) ([]*domain.Partner, error)

	// Detail bundles the partner org + aggregates in both directions
	// (caller-as-supplier and caller-as-buyer) + recent shared orders.
	Detail(ctx context.Context, callerOrgID, partnerOrgID uuid.UUID) (*domain.PartnerDetail, error)
}

type partnerService struct {
	partnerRepo repository.PartnerRepository
	orgRepo     domain.OrganizationRepository
}

func NewPartnerService(p repository.PartnerRepository, o domain.OrganizationRepository) PartnerService {
	return &partnerService{partnerRepo: p, orgRepo: o}
}

func (s *partnerService) List(ctx context.Context, callerOrgID uuid.UUID, role string) ([]*domain.Partner, error) {
	return s.partnerRepo.ListByRole(ctx, callerOrgID, role)
}

func (s *partnerService) Detail(ctx context.Context, callerOrgID, partnerOrgID uuid.UUID) (*domain.PartnerDetail, error) {
	// Self-org is allowed — happens with internal-only orders where buyer
	// and supplier are the same org. Aggregates will reflect that.
	org, err := s.orgRepo.GetByID(ctx, partnerOrgID)
	if err != nil {
		return nil, err
	}

	detail := &domain.PartnerDetail{Organization: org}

	// Caller as buyer → partner is supplier. (orgs we bought from)
	if p, err := s.partnerRepo.GetRelationship(ctx, callerOrgID, partnerOrgID, "supplier"); err == nil {
		detail.AsSupplier = p
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	// Caller as supplier → partner is buyer. (orgs that bought from us)
	if p, err := s.partnerRepo.GetRelationship(ctx, callerOrgID, partnerOrgID, "buyer"); err == nil {
		detail.AsBuyer = p
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	// No relationship in either direction means the partner UUID doesn't
	// correspond to any order this org participated in.
	if detail.AsSupplier == nil && detail.AsBuyer == nil {
		return nil, domain.ErrNotFound
	}

	orders, err := s.partnerRepo.RecentOrdersWith(ctx, callerOrgID, partnerOrgID, 20)
	if err != nil {
		return nil, err
	}
	detail.RecentOrders = orders
	return detail, nil
}
