package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type MarketplaceHandler struct {
	service service.MarketplaceService
}

func NewMarketplaceHandler(s service.MarketplaceService) *MarketplaceHandler {
	return &MarketplaceHandler{service: s}
}

// --- Listings ---

func (h *MarketplaceHandler) ListByOrg(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	listings, err := h.service.ListByOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, listings)
}

func (h *MarketplaceHandler) CreateListing(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req domain.MarketplaceListing
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.CreateListing(r.Context(), &req, orgID); err != nil {
		if err == domain.ErrConflict {
			writeError(w, http.StatusConflict, "listing already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, req)
}

func (h *MarketplaceHandler) UpdateListing(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid listing id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req domain.MarketplaceListing
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id

	if err := h.service.UpdateListing(r.Context(), &req, orgID); err != nil {
		if err == domain.ErrNotFound {
			writeError(w, http.StatusNotFound, "listing not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, req)
}

func (h *MarketplaceHandler) DeleteListing(w http.ResponseWriter, r *http.Request) {
	orgID, ok := middleware.GetOrgIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing org context")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid listing id")
		return
	}

	if err := h.service.DeleteListing(r.Context(), id, orgID); err != nil {
		if err == domain.ErrNotFound {
			writeError(w, http.StatusNotFound, "listing not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "listing deleted"})
}

// --- Search (Public) ---

func (h *MarketplaceHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")

	var lat, lng, radius *float64
	if latStr := r.URL.Query().Get("lat"); latStr != "" {
		if val, err := strconv.ParseFloat(latStr, 64); err == nil {
			lat = &val
		}
	}
	if lngStr := r.URL.Query().Get("lng"); lngStr != "" {
		if val, err := strconv.ParseFloat(lngStr, 64); err == nil {
			lng = &val
		}
	}
	if radiusStr := r.URL.Query().Get("radius"); radiusStr != "" {
		if val, err := strconv.ParseFloat(radiusStr, 64); err == nil {
			radius = &val
		}
	}

	filters := make(map[string]any)
	if minStr := r.URL.Query().Get("min_price"); minStr != "" {
		if val, err := strconv.ParseInt(minStr, 10, 64); err == nil {
			filters["min_price"] = val
		}
	}
	if maxStr := r.URL.Query().Get("max_price"); maxStr != "" {
		if val, err := strconv.ParseInt(maxStr, 10, 64); err == nil {
			filters["max_price"] = val
		}
	}

	listings, err := h.service.Search(r.Context(), q, lat, lng, radius, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if listings == nil {
		listings = []*domain.MarketplaceListing{}
	}

	writeJSON(w, http.StatusOK, listings)
}

// --- Cart ---

func (h *MarketplaceHandler) getActiveCart(r *http.Request) (*domain.Cart, error) {
	orgID, hasOrg := middleware.GetOrgIDFromContext(r.Context())

	var buyerOrgID *uuid.UUID
	var customerID *uuid.UUID

	if hasOrg {
		buyerOrgID = &orgID
	} else {
		// Try to get customer ID if they are B2C (Assuming we use UserID for customers for now)
		if userIDStr, ok := middleware.GetUserIDFromContext(r.Context()); ok {
			if uid, err := uuid.Parse(userIDStr); err == nil {
				customerID = &uid
			}
		}
	}

	if buyerOrgID == nil && customerID == nil {
		return nil, domain.ErrUnauthorized
	}

	return h.service.GetOrCreateCart(r.Context(), buyerOrgID, customerID)
}

func (h *MarketplaceHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	cart, err := h.getActiveCart(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unable to determine cart owner")
		return
	}

	fullCart, err := h.service.GetCart(r.Context(), cart.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			fullCart = &domain.Cart{
				ID:    cart.ID,
				Items: []domain.CartItem{},
			}
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if fullCart.Items == nil {
		fullCart.Items = []domain.CartItem{}
	}

	writeJSON(w, http.StatusOK, fullCart)
}

type cartItemReq struct {
	ListingID uuid.UUID `json:"listing_id"`
	Quantity  int       `json:"quantity"`
}

func (h *MarketplaceHandler) AddToCart(w http.ResponseWriter, r *http.Request) {
	cart, err := h.getActiveCart(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unable to determine cart owner")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req cartItemReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Quantity < 1 {
		writeError(w, http.StatusBadRequest, "quantity must be at least 1")
		return
	}

	if err := h.service.AddToCart(r.Context(), cart.ID, req.ListingID, req.Quantity); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "item added to cart"})
}

func (h *MarketplaceHandler) UpdateCartItem(w http.ResponseWriter, r *http.Request) {
	cart, err := h.getActiveCart(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unable to determine cart owner")
		return
	}

	listingIDStr := chi.URLParam(r, "listing_id")
	listingID, err := uuid.Parse(listingIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid listing id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Quantity < 1 {
		writeError(w, http.StatusBadRequest, "quantity must be at least 1")
		return
	}

	if err := h.service.UpdateCartItem(r.Context(), cart.ID, listingID, req.Quantity); err != nil {
		if err == domain.ErrNotFound {
			writeError(w, http.StatusNotFound, "cart item not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "cart item updated"})
}

func (h *MarketplaceHandler) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	cart, err := h.getActiveCart(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unable to determine cart owner")
		return
	}

	listingIDStr := chi.URLParam(r, "listing_id")
	listingID, err := uuid.Parse(listingIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid listing id")
		return
	}

	if err := h.service.RemoveFromCart(r.Context(), cart.ID, listingID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "item removed"})
}

type checkoutReq struct {
	DeliveryAddressID *uuid.UUID `json:"delivery_address_id"`
}

func (h *MarketplaceHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	cart, err := h.getActiveCart(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unable to determine cart owner")
		return
	}

	orgID, _ := middleware.GetOrgIDFromContext(r.Context())
	userIDStr, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid user context")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req checkoutReq
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	orders, err := h.service.Checkout(r.Context(), cart.ID, req.DeliveryAddressID, orgID, userID)
	if err != nil {
		if err.Error() == "cart is empty" {
			writeError(w, http.StatusBadRequest, "cart is empty")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message": "checkout successful",
		"orders":  orders,
	})
}
