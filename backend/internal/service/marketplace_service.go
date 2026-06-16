package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MarketplaceService interface {
	// Listings
	CreateListing(ctx context.Context, listing *domain.MarketplaceListing, orgID uuid.UUID) error
	UpdateListing(ctx context.Context, listing *domain.MarketplaceListing, orgID uuid.UUID) error
	DeleteListing(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.MarketplaceListing, error)
	Search(ctx context.Context, query string, lat, lng, radiusKM *float64, filters map[string]any) ([]*domain.MarketplaceListing, error)

	// Cart
	GetOrCreateCart(ctx context.Context, buyerOrgID, customerID *uuid.UUID) (*domain.Cart, error)
	AddToCart(ctx context.Context, cartID, listingID uuid.UUID, quantity int) error
	UpdateCartItem(ctx context.Context, cartID, listingID uuid.UUID, quantity int) error
	RemoveFromCart(ctx context.Context, cartID, listingID uuid.UUID) error
	GetCart(ctx context.Context, cartID uuid.UUID) (*domain.Cart, error)

	// Checkout — most complex, atomic
	Checkout(ctx context.Context, cartID uuid.UUID, deliveryAddressID *uuid.UUID, orgID uuid.UUID, userID uuid.UUID) ([]*domain.Order, error)
}

type marketplaceService struct {
	marketRepo   repository.MarketplaceRepository
	invRepo      repository.InventoryRepository
	orderRepo    repository.OrderRepository
	productRepo  repository.ProductRepository
	locationRepo repository.LocationRepository
	cache        cache.Cache
	bus          events.Bus
	pool         *pgxpool.Pool
}

func NewMarketplaceService(
	marketRepo repository.MarketplaceRepository,
	invRepo repository.InventoryRepository,
	orderRepo repository.OrderRepository,
	productRepo repository.ProductRepository,
	locationRepo repository.LocationRepository,
	c cache.Cache,
	bus events.Bus,
	pool *pgxpool.Pool,
) MarketplaceService {
	return &marketplaceService{
		marketRepo:   marketRepo,
		invRepo:      invRepo,
		orderRepo:    orderRepo,
		productRepo:  productRepo,
		locationRepo: locationRepo,
		cache:        c,
		bus:          bus,
		pool:         pool,
	}
}

func (s *marketplaceService) invalidate(ctx context.Context, orgID uuid.UUID) {
	_ = s.cache.DeleteByPattern(ctx, cache.ListingsByOrgPattern(orgID))
	_ = s.cache.DeleteByPattern(ctx, cache.MarketplaceSearchPattern())
}

// --- Listings ---

func (s *marketplaceService) CreateListing(ctx context.Context, listing *domain.MarketplaceListing, orgID uuid.UUID) error {
	// Validate price
	if listing.ListingPrice < 0 {
		return errors.New("listing price must be non-negative")
	}
	if listing.MinOrderQty < 1 {
		listing.MinOrderQty = 1
	}
	if listing.MaxOrderQty != nil && *listing.MaxOrderQty < listing.MinOrderQty {
		return errors.New("max_order_qty must be >= min_order_qty")
	}

	// Verify product belongs to org
	if _, err := s.productRepo.GetByID(ctx, listing.ProductID, orgID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return errors.New("product not found in your organization")
		}
		return err
	}

	// Verify location belongs to org (if specified)
	if listing.LocationID != nil {
		if _, err := s.locationRepo.GetByID(ctx, *listing.LocationID, orgID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return errors.New("location not found in your organization")
			}
			return err
		}
	}

	listing.ID = uuid.New()
	listing.OrgID = orgID
	listing.IsActive = true
	now := time.Now()
	listing.CreatedAt = now
	listing.UpdatedAt = now
	if err := s.marketRepo.CreateListing(ctx, listing); err != nil {
		return err
	}
	s.invalidate(ctx, orgID)
	_ = s.bus.Publish(ctx, events.NewEvent(events.TopicListingCreated, orgID.String(), "", map[string]any{
		"id":            listing.ID,
		"product_id":    listing.ProductID,
		"listing_price": listing.ListingPrice,
	}))
	return nil
}

func (s *marketplaceService) UpdateListing(ctx context.Context, listing *domain.MarketplaceListing, orgID uuid.UUID) error {
	listing.OrgID = orgID
	listing.UpdatedAt = time.Now()
	if err := s.marketRepo.UpdateListing(ctx, listing); err != nil {
		return err
	}
	s.invalidate(ctx, orgID)
	_ = s.bus.Publish(ctx, events.NewEvent(events.TopicListingUpdated, orgID.String(), "", map[string]any{
		"id": listing.ID,
	}))
	return nil
}

func (s *marketplaceService) DeleteListing(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	if err := s.marketRepo.DeleteListing(ctx, id, orgID); err != nil {
		return err
	}
	s.invalidate(ctx, orgID)
	_ = s.bus.Publish(ctx, events.NewEvent(events.TopicListingDeleted, orgID.String(), "", map[string]any{
		"id": id,
	}))
	return nil
}

func (s *marketplaceService) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*domain.MarketplaceListing, error) {
	key := cache.ListingsByOrgKey(orgID)
	var cached []*domain.MarketplaceListing
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	list, err := s.marketRepo.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, list, cache.TTLMedium)
	return list, nil
}

func (s *marketplaceService) Search(ctx context.Context, query string, lat, lng, radiusKM *float64, filters map[string]any) ([]*domain.MarketplaceListing, error) {
	// Build a canonical fingerprint of the request so identical searches share a key.
	fingerprint := fmt.Sprintf("q=%q|lat=%v|lng=%v|r=%v|f=%v", query, lat, lng, radiusKM, filters)
	key := cache.MarketplaceSearchKey(fingerprint)

	var cached []*domain.MarketplaceListing
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	results, err := s.marketRepo.Search(ctx, query, lat, lng, radiusKM, filters)
	if err != nil {
		return nil, err
	}
	_ = s.cache.Set(ctx, key, results, cache.TTLShort)
	return results, nil
}

// --- Cart ---

func (s *marketplaceService) GetOrCreateCart(ctx context.Context, buyerOrgID, customerID *uuid.UUID) (*domain.Cart, error) {
	cart, err := s.marketRepo.GetActiveCart(ctx, buyerOrgID, customerID)
	if err == nil {
		return cart, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	// Create new cart
	newCart := &domain.Cart{
		ID:         uuid.New(),
		BuyerOrgID: buyerOrgID,
		CustomerID: customerID,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		CreatedAt:  time.Now(),
	}
	if err := s.marketRepo.CreateCart(ctx, newCart); err != nil {
		return nil, err
	}
	return newCart, nil
}

func (s *marketplaceService) AddToCart(ctx context.Context, cartID, listingID uuid.UUID, quantity int) error {
	if quantity < 1 {
		return errors.New("quantity must be at least 1")
	}

	listing, err := s.marketRepo.GetListingByID(ctx, listingID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return errors.New("listing not found")
		}
		return err
	}
	if !listing.IsActive {
		return errors.New("listing is no longer available")
	}
	if quantity < listing.MinOrderQty {
		return fmt.Errorf("minimum order quantity is %d", listing.MinOrderQty)
	}
	if listing.MaxOrderQty != nil && quantity > *listing.MaxOrderQty {
		return fmt.Errorf("maximum order quantity is %d", *listing.MaxOrderQty)
	}

	// Stock check
	inv, err := s.invRepo.GetByProductID(ctx, listing.ProductID, listing.OrgID)
	if err == nil && inv.Quantity < quantity {
		return fmt.Errorf("only %d in stock", inv.Quantity)
	}

	item := &domain.CartItem{
		ID:        uuid.New(),
		CartID:    cartID,
		ListingID: listingID,
		Quantity:  quantity,
		AddedAt:   time.Now(),
	}
	return s.marketRepo.AddCartItem(ctx, item)
}

func (s *marketplaceService) UpdateCartItem(ctx context.Context, cartID, listingID uuid.UUID, quantity int) error {
	if quantity < 1 {
		return errors.New("quantity must be at least 1")
	}

	listing, err := s.marketRepo.GetListingByID(ctx, listingID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return errors.New("listing not found")
		}
		return err
	}
	if listing.MaxOrderQty != nil && quantity > *listing.MaxOrderQty {
		return fmt.Errorf("maximum order quantity is %d", *listing.MaxOrderQty)
	}
	if quantity < listing.MinOrderQty {
		return fmt.Errorf("minimum order quantity is %d", listing.MinOrderQty)
	}

	// Stock check
	inv, err := s.invRepo.GetByProductID(ctx, listing.ProductID, listing.OrgID)
	if err == nil && inv.Quantity < quantity {
		return fmt.Errorf("only %d in stock", inv.Quantity)
	}

	item := &domain.CartItem{
		CartID:    cartID,
		ListingID: listingID,
		Quantity:  quantity,
	}
	return s.marketRepo.UpdateCartItem(ctx, item)
}

func (s *marketplaceService) RemoveFromCart(ctx context.Context, cartID, listingID uuid.UUID) error {
	return s.marketRepo.RemoveCartItem(ctx, cartID, listingID)
}

func (s *marketplaceService) GetCart(ctx context.Context, cartID uuid.UUID) (*domain.Cart, error) {
	return s.marketRepo.GetCartWithItems(ctx, cartID)
}

// --- Checkout ---

func (s *marketplaceService) Checkout(ctx context.Context, cartID uuid.UUID, deliveryAddressID *uuid.UUID, buyerOrgID uuid.UUID, userID uuid.UUID) ([]*domain.Order, error) {
	// 1. Get cart + items
	cart, err := s.marketRepo.GetCartWithItems(ctx, cartID)
	if err != nil {
		return nil, err
	}
	if len(cart.Items) == 0 {
		return nil, errors.New("cart is empty")
	}

	// 2. Group items by listing.OrgID (supplier)
	supplierGroups := make(map[uuid.UUID][]domain.CartItem)
	for _, item := range cart.Items {
		if item.Listing == nil {
			return nil, errors.New("cart item missing listing data")
		}
		supplierGroups[item.Listing.OrgID] = append(supplierGroups[item.Listing.OrgID], item)
	}

	// 3. Begin global transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txMarketRepo := s.marketRepo.WithTx(tx)
	txInvRepo := s.invRepo.WithTx(tx)
	txOrderRepo := s.orderRepo.WithTx(tx)

	var createdOrders []*domain.Order

	// 4. Process each supplier group.
	//    Order: validate stock → create Order (placeholder totals) → loop items
	//    (inventory decrement + reservation + order_item) → update Order with
	//    final totals. We MUST create the order before reservations/items
	//    because both have FKs to orders.id.
	for supplierID, items := range supplierGroups {
		orderType := "b2b"
		now := time.Now()
		orderID := uuid.New()

		// Pre-flight: validate all stock before mutating anything
		for _, item := range items {
			inventory, err := txInvRepo.GetByProductID(ctx, item.Listing.ProductID, supplierID)
			if err != nil {
				return nil, fmt.Errorf("inventory lookup failed for product %s: %w", item.Listing.ProductID, err)
			}
			if inventory.Quantity < item.Quantity {
				return nil, fmt.Errorf("insufficient stock for %s: need %d, have %d",
					item.Listing.ProductName, item.Quantity, inventory.Quantity)
			}
		}

		// Create the Order row first so reservations + order_items have a parent.
		order := &domain.Order{
			ID:                orderID,
			OrgID:             buyerOrgID,
			UserID:            userID,
			Status:            domain.OrderStatusPending,
			TotalAmount:       0,
			CreatedAt:         now,
			UpdatedAt:         now,
			OrderType:         orderType,
			BuyerOrgID:        &buyerOrgID,
			SupplierOrgID:     &supplierID,
			DeliveryAddressID: deliveryAddressID,
			Subtotal:          0,
			PaymentStatus:     "unpaid",
		}
		if err := txOrderRepo.Create(ctx, order); err != nil {
			return nil, fmt.Errorf("create order failed: %w", err)
		}

		var subtotal int64
		for _, item := range items {
			inventory, err := txInvRepo.GetByProductID(ctx, item.Listing.ProductID, supplierID)
			if err != nil {
				return nil, err
			}

			// Decrement stock
			inventory.Quantity -= item.Quantity
			inventory.UpdatedAt = now
			if err := txInvRepo.Update(ctx, inventory); err != nil {
				return nil, fmt.Errorf("inventory update failed: %w", err)
			}

			// Reservation now has a real Order to point at.
			reservation := &domain.InventoryReservation{
				ID:          uuid.New(),
				InventoryID: inventory.ID,
				OrderID:     &orderID,
				OrgID:       supplierID,
				Quantity:    item.Quantity,
				Status:      "committed",
				ReservedAt:  now,
				ExpiresAt:   now.Add(30 * time.Minute),
			}
			if err := txMarketRepo.CreateReservation(ctx, reservation); err != nil {
				return nil, fmt.Errorf("reservation create failed: %w", err)
			}

			// Order item
			orderItem := &domain.OrderItem{
				ID:        uuid.New(),
				OrgID:     buyerOrgID,
				OrderID:   orderID,
				ProductID: item.Listing.ProductID,
				Quantity:  item.Quantity,
				UnitPrice: item.Listing.ListingPrice,
			}
			if err := txOrderRepo.CreateOrderItem(ctx, orderItem); err != nil {
				return nil, fmt.Errorf("order item create failed: %w", err)
			}

			subtotal += item.Listing.ListingPrice * int64(item.Quantity)
		}

		// Update the order with the final totals.
		order.Subtotal = subtotal
		order.TotalAmount = subtotal
		order.UpdatedAt = time.Now()
		if err := txOrderRepo.Update(ctx, order); err != nil {
			return nil, fmt.Errorf("order total update failed: %w", err)
		}

		createdOrders = append(createdOrders, order)
	}

	// Clean up cart after successful checkout processing
	// We can simply clear the cart items
	for _, item := range cart.Items {
		if err := txMarketRepo.RemoveCartItem(ctx, cartID, item.ListingID); err != nil {
			return nil, err
		}
	}

	// 5. Commit all
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return createdOrders, nil
}
