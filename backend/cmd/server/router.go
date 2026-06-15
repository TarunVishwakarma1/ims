package main

import (
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/config"
	"github.com/TarunVishwakarma1/ims/backend/internal/handler"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/metrics"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/rbac"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(
	authH *handler.AuthHandler,
	userH *handler.UserHandler,
	categoryH *handler.CategoryHandler,
	productH *handler.ProductHandler,
	inventoryH *handler.InventoryHandler,
	orderH *handler.OrderHandler,
	roleH *handler.RoleHandler,
	locationH *handler.LocationHandler,
	marketH *handler.MarketplaceHandler,
	eventsH *handler.EventsHandler,
	paymentH *handler.PaymentHandler,
	webhookH *handler.WebhookHandler,
	cfg *config.Config,
	pool *pgxpool.Pool,
	cacheClient cache.Cache,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware (applied to ALL routes)
	r.Use(chiMiddleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders())
	r.Use(metrics.HTTPMiddleware) // record request count + latency
	r.Use(middleware.RateLimiter())
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// Public routes (no auth)
	r.Get("/health", handler.HealthCheck(pool, cacheClient))

	// Prometheus metrics — optional bearer-token auth
	if cfg.MetricsEnabled {
		r.Handle("/metrics", metrics.Handler(cfg.MetricsToken))
	}

	// Auth routes — stricter rate limit (anti brute-force)
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthRateLimiter())
		r.Post("/api/auth/register", authH.Signup)
		r.Post("/api/auth/login", authH.Login)
		r.Post("/api/auth/refresh", authH.RefreshToken)
		r.Post("/api/auth/logout", authH.Logout)
	})

	// Marketplace Search (Public)
	r.Get("/api/marketplace/search", marketH.Search)

	// Webhook receivers (public — HMAC-verified)
	r.Post("/api/webhooks/razorpay", webhookH.RazorpayWebhook)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))

		// Real-time event stream (SSE)
		r.Get("/api/events", eventsH.Stream)

		// Payments
		r.Get("/api/payments/config", paymentH.Config)
		r.Post("/api/payments/orders", paymentH.CreateOrder)
		r.Get("/api/payments", paymentH.ListPayments)
		r.Get("/api/payments/{id}", paymentH.GetPayment)
		// Mock-only endpoints — service rejects when mockMode = false
		r.Post("/api/payments/mock/capture", paymentH.MockCapture)
		r.Post("/api/payments/mock/fail", paymentH.MockFail)
		// DLQ inspection (admin)
		r.With(middleware.RequirePermission(rbac.UsersDelete)).Get("/api/payments/webhooks/dlq", paymentH.ListDLQ)

		// Email verification (authenticated user only)
		r.Post("/api/auth/verify-email", authH.VerifyEmail)
		r.Post("/api/auth/resend-verification", authH.ResendVerification)

		// Users
		r.With(middleware.RequirePermission(rbac.UsersView)).Get("/api/users", userH.ListUsers)
		r.With(middleware.RequirePermission(rbac.UsersCreate)).Post("/api/users", userH.CreateUser)
		r.With(middleware.RequirePermission(rbac.UsersView)).Get("/api/users/{id}", userH.GetUser)
		r.With(middleware.RequirePermission(rbac.UsersEdit)).Put("/api/users/{id}", userH.UpdateUser)
		r.With(middleware.RequirePermission(rbac.UsersDelete)).Delete("/api/users/{id}", userH.DeleteUser)

		// Categories
		r.With(middleware.RequirePermission(rbac.CategoriesManage)).Post("/api/categories", categoryH.CreateCategory)
		r.Get("/api/categories", categoryH.ListCategories)
		r.Get("/api/categories/{id}", categoryH.GetCategory)
		r.With(middleware.RequirePermission(rbac.CategoriesManage)).Put("/api/categories/{id}", categoryH.UpdateCategory)
		r.With(middleware.RequirePermission(rbac.CategoriesManage)).Delete("/api/categories/{id}", categoryH.DeleteCategory)

		// Products
		r.With(middleware.RequirePermission(rbac.ProductsManage)).Post("/api/products", productH.CreateProduct)
		r.Get("/api/products", productH.ListProducts)
		r.Get("/api/products/{id}", productH.GetProduct)
		r.With(middleware.RequirePermission(rbac.ProductsManage)).Put("/api/products/{id}", productH.UpdateProduct)
		r.With(middleware.RequirePermission(rbac.ProductsManage)).Delete("/api/products/{id}", productH.DeleteProduct)
		r.Get("/api/categories/{category_id}/products", productH.ListByCategory)

		// Inventory
		r.With(middleware.RequirePermission(rbac.InventoryManage)).Post("/api/inventory", inventoryH.CreateInventory)
		r.Get("/api/inventory", inventoryH.ListInventory)
		r.Get("/api/inventory/low-stock", inventoryH.ListLowStock)
		r.Get("/api/inventory/product/{product_id}", inventoryH.GetInventoryByProduct)
		r.With(middleware.RequirePermission(rbac.InventoryManage)).Put("/api/inventory/{id}", inventoryH.UpdateInventory)

		// Orders
		r.Post("/api/orders", orderH.CreateOrder)
		r.Get("/api/orders", orderH.ListOrders)
		r.Get("/api/orders/{id}", orderH.GetOrder)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Put("/api/orders/{id}/status", orderH.UpdateStatus)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Delete("/api/orders/{id}", orderH.DeleteOrder)
		r.Get("/api/orders/{id}/items", orderH.GetOrderItems)
		r.Get("/api/users/{user_id}/orders", orderH.ListUserOrders)
		// Roles & Permissions
		r.With(middleware.RequirePermission(rbac.RolesManage)).Get("/api/roles", roleH.ListRoles)
		r.With(middleware.RequirePermission(rbac.RolesManage)).Post("/api/roles", roleH.CreateRole)
		r.With(middleware.RequirePermission(rbac.RolesManage)).Put("/api/roles/{id}/permissions", roleH.UpdateRolePermissions)
		r.With(middleware.RequirePermission(rbac.RolesManage)).Put("/api/roles/{id}", roleH.UpdateRole)
		r.With(middleware.RequirePermission(rbac.RolesManage)).Delete("/api/roles/{id}", roleH.DeleteRole)
		r.With(middleware.RequirePermission(rbac.RolesManage)).Get("/api/permissions", roleH.ListPermissions)
		r.With(middleware.RequirePermission(rbac.RolesManage)).Post("/api/roles/reload", roleH.ReloadPermissions)

		// Locations
		r.With(middleware.RequirePermission(rbac.LocationsManage)).Get("/api/locations", locationH.List)
		r.With(middleware.RequirePermission(rbac.LocationsManage)).Post("/api/locations", locationH.Create)
		r.With(middleware.RequirePermission(rbac.LocationsManage)).Put("/api/locations/{id}", locationH.Update)
		r.With(middleware.RequirePermission(rbac.LocationsManage)).Delete("/api/locations/{id}", locationH.Delete)

		// Marketplace Listings
		r.Get("/api/listings", marketH.ListByOrg)
		r.Post("/api/listings", marketH.CreateListing)
		r.Put("/api/listings/{id}", marketH.UpdateListing)
		r.Delete("/api/listings/{id}", marketH.DeleteListing)

		// Marketplace Cart
		r.Get("/api/cart", marketH.GetCart)
		r.Post("/api/cart/items", marketH.AddToCart)
		r.Put("/api/cart/items/{listing_id}", marketH.UpdateCartItem)
		r.Delete("/api/cart/items/{listing_id}", marketH.RemoveFromCart)
		r.Post("/api/cart/checkout", marketH.Checkout)
	})

	return r
}
