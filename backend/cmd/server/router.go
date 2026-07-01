package main

import (
	"net/http"
	"strings"

	"github.com/TarunVishwakarma1/ims/backend/config"
	"github.com/TarunVishwakarma1/ims/backend/internal/handler"
	shophandler "github.com/TarunVishwakarma1/ims/backend/internal/handler/shop"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/metrics"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/rbac"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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
	partnerH *handler.PartnerHandler,
	returnH *handler.ReturnHandler,
	notificationH *handler.NotificationHandler,
	auditH *handler.AuditHandler,
	totpH *handler.TOTPHandler,
	couponH *handler.CouponHandler,
	cfg *config.Config,
	pool *pgxpool.Pool,
	cacheClient cache.Cache,
	shopEnabled bool,
	shopAuthH *shophandler.AuthHandler,
	shopCustH *shophandler.CustomerHandler,
	shopCartH *shophandler.CartHandler,
	shopCheckH *shophandler.CheckoutHandler,
	shopCatalogH *shophandler.CatalogHandler,
	adminBannerH *handler.AdminBannerHandler,
	shopBannerH *shophandler.BannerHandler,
	shopOrderH *shophandler.OrderHandler,
	shopPaymentH *shophandler.PaymentHandler,
	adminShopOrderH *shophandler.AdminOrderHandler,
	shopDirectoryH *shophandler.DirectoryHandler,
	shopResolve middleware.ShopResolver,
	uploadDir string,
	profileH *shophandler.ProfileHandler,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware (applied to ALL routes)
	r.Use(chiMiddleware.RequestID)
	r.Use(middleware.RealIP)
	// Structured request logger (zap JSON) replaces chi's stdlib text logger
	// so file logs are uniformly JSON and greppable via `jq` / aggregators.
	r.Use(middleware.RequestLogger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders(cfg.ENV))
	r.Use(metrics.HTTPMiddleware) // record request count + latency
	r.Use(middleware.RateLimiter())
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(chiMiddleware.Compress(5, "application/json", "text/html", "text/plain", "text/css", "application/javascript"))

	// Static file server — serves uploaded banner images (shop only)
	if shopEnabled {
		r.Mount("/uploads/", bannerUploadsHandler(uploadDir))
	}

	// Public routes (no auth)
	r.Get("/health", handler.HealthCheck(pool, cacheClient))

	// Prometheus metrics — optional bearer-token auth
	if cfg.MetricsEnabled {
		r.Handle("/metrics", metrics.Handler(cfg.MetricsToken))
	}

	// Auth routes — stricter rate limit (anti brute-force)
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthRateLimiter(cfg.ENV))
		r.Post("/api/auth/register", authH.Signup)
		r.Post("/api/auth/login", authH.Login)
		r.Post("/api/auth/login/verify-2fa", totpH.VerifyLogin)
		r.Post("/api/auth/login/resend-2fa", totpH.ResendLoginOTP)
		r.Post("/api/auth/password-reset/request", authH.RequestPasswordReset)
		r.Post("/api/auth/password-reset/confirm", authH.ConfirmPasswordReset)
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
		r.With(middleware.Idempotency(pool)).Post("/api/payments/orders", paymentH.CreateOrder)
		r.With(middleware.RequirePermission(rbac.PaymentsView)).Get("/api/payments", paymentH.ListPayments)
		r.With(middleware.RequirePermission(rbac.PaymentsView)).Get("/api/payments/export.csv", paymentH.ExportPayments)

		// Coupons — supplier-scoped discount codes.
		r.With(middleware.RequirePermission(rbac.CouponsView)).Get("/api/coupons", couponH.List)
		r.With(middleware.RequirePermission(rbac.CouponsView)).Get("/api/coupons/{id}", couponH.GetByID)
		r.With(middleware.RequirePermission(rbac.CouponsManage)).Post("/api/coupons", couponH.Create)
		r.With(middleware.RequirePermission(rbac.CouponsManage)).Put("/api/coupons/{id}", couponH.Update)
		r.With(middleware.RequirePermission(rbac.CouponsManage)).Delete("/api/coupons/{id}", couponH.Delete)
		// Validate is open to any authed user — cart needs to preview the discount.
		r.Post("/api/coupons/validate", couponH.Validate)
		r.Get("/api/payments/{id}", paymentH.GetPayment)
		r.Get("/api/payments/{id}/receipt", paymentH.Receipt)
		r.Get("/api/payments/{id}/receipt.pdf", paymentH.ReceiptPDF)
		r.Get("/api/payments/{id}/refunds", paymentH.ListRefunds)
		r.With(middleware.RequirePermission(rbac.PaymentsRefund)).Post("/api/payments/{id}/refund", paymentH.RefundPayment)
		// Mock-only endpoints — service rejects when mockMode = false
		r.Post("/api/payments/mock/capture", paymentH.MockCapture)
		r.Post("/api/payments/mock/fail", paymentH.MockFail)
		// Audit log viewer (admin / compliance)
		r.With(middleware.RequirePermission(rbac.AuditView)).Get("/api/audit", auditH.List)

		// Notifications outbox / DLQ admin
		r.With(middleware.RequirePermission(rbac.NotificationsView)).Get("/api/notifications", notificationH.List)
		r.With(middleware.RequirePermission(rbac.NotificationsView)).Get("/api/notifications/{id}", notificationH.GetByID)
		r.With(middleware.RequirePermission(rbac.NotificationsManage)).Post("/api/notifications/{id}/resend", notificationH.Resend)

		// Webhook event log + DLQ
		r.With(middleware.RequirePermission(rbac.WebhooksView)).Get("/api/payments/webhooks/events", paymentH.ListEvents)
		r.With(middleware.RequirePermission(rbac.WebhooksManage)).Get("/api/payments/webhooks/secrets", paymentH.WebhookSecretStatus)
		r.With(middleware.RequirePermission(rbac.WebhooksView)).Get("/api/payments/webhooks/dlq", paymentH.ListDLQ)
		r.With(middleware.RequirePermission(rbac.WebhooksView)).Get("/api/payments/webhooks/dlq/{id}", paymentH.GetDLQEvent)
		r.With(middleware.RequirePermission(rbac.WebhooksManage)).Post("/api/payments/webhooks/dlq/{id}/replay", paymentH.ReplayDLQ)

		// Email verification (authenticated user only)
		r.Post("/api/auth/verify-email", authH.VerifyEmail)
		r.Post("/api/auth/resend-verification", authH.ResendVerification)
		r.Get("/api/auth/me", authH.Me)

		// 2FA enrollment (requires existing session — user opts in
		// from settings while logged in).
		r.Post("/api/auth/2fa/enroll", totpH.Enroll)
		r.Post("/api/auth/2fa/confirm", totpH.Confirm)
		r.Post("/api/auth/2fa/disable", totpH.Disable)
		r.Post("/api/auth/2fa/email/enable", totpH.EnableEmail2FA)
		r.Post("/api/auth/2fa/email/disable", totpH.DisableEmail2FA)

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
		r.Get("/api/inventory/{id}", inventoryH.GetByID)
		r.Get("/api/inventory/{id}/timeline", inventoryH.Timeline)
		r.With(middleware.RequirePermission(rbac.InventoryManage)).Put("/api/inventory/{id}", inventoryH.UpdateInventory)

		// Orders
		r.Post("/api/orders", orderH.CreateOrder)
		r.Get("/api/orders", orderH.ListOrders)
		r.Get("/api/orders/export", orderH.ExportOrders)
		r.Get("/api/orders/{id}", orderH.GetOrder)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Put("/api/orders/{id}/status", orderH.UpdateStatus)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Post("/api/orders/bulk-status", orderH.BulkUpdateStatus)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Post("/api/orders/{id}/cancel", orderH.CancelOrder)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Delete("/api/orders/{id}", orderH.DeleteOrder)
		r.Get("/api/orders/{id}/items", orderH.GetOrderItems)
		r.Get("/api/orders/{id}/timeline", orderH.GetOrderTimeline)
		r.Get("/api/orders/{id}/cancel-preview", orderH.CancelPreview)
		r.Get("/api/orders/{id}/invoice.pdf", orderH.Invoice)
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

		// Partners (suppliers / buyers — derived from orders)
		r.Get("/api/partners", partnerH.List)
		r.Get("/api/partners/{id}", partnerH.Detail)

		// Returns / RMA
		r.With(middleware.RequirePermission(rbac.OrdersView)).Get("/api/returns", returnH.ListAll)
		r.With(middleware.RequirePermission(rbac.OrdersView)).Get("/api/returns/{id}", returnH.GetByID)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Post("/api/returns/{id}/approve", returnH.Approve)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Post("/api/returns/{id}/reject", returnH.Reject)
		r.With(middleware.RequirePermission(rbac.OrdersManage)).Post("/api/returns/{id}/received", returnH.MarkReceived)
		r.With(middleware.RequirePermission(rbac.OrdersCreate)).Post("/api/orders/{id}/returns", returnH.Create)
		r.With(middleware.RequirePermission(rbac.OrdersView)).Get("/api/orders/{id}/returns", returnH.ListByOrder)

		// Marketplace Cart
		r.Get("/api/cart", marketH.GetCart)
		r.Post("/api/cart/items", marketH.AddToCart)
		r.Put("/api/cart/items/{listing_id}", marketH.UpdateCartItem)
		r.Delete("/api/cart/items/{listing_id}", marketH.RemoveFromCart)
		r.Post("/api/cart/checkout", marketH.Checkout)

		// Admin banner CMS (only registered when shop is enabled to avoid nil handler panic)
		if shopEnabled {
			r.With(middleware.RequirePermission(rbac.BannersManage)).Post("/api/admin/banners/upload", adminBannerH.Upload)
			r.With(middleware.RequirePermission(rbac.BannersView)).Get("/api/admin/banners", adminBannerH.List)
			r.With(middleware.RequirePermission(rbac.BannersManage)).Post("/api/admin/banners", adminBannerH.Create)
			r.With(middleware.RequirePermission(rbac.BannersView)).Get("/api/admin/banners/{id}", adminBannerH.Get)
			r.With(middleware.RequirePermission(rbac.BannersManage)).Patch("/api/admin/banners/{id}", adminBannerH.Update)
			r.With(middleware.RequirePermission(rbac.BannersManage)).Post("/api/admin/banners/{id}/publish", adminBannerH.Publish)
			r.With(middleware.RequirePermission(rbac.BannersManage)).Post("/api/admin/banners/{id}/archive", adminBannerH.Archive)
			r.With(middleware.RequirePermission(rbac.BannersManage)).Delete("/api/admin/banners/{id}", adminBannerH.Delete)

			// Admin B2C order management (list + advance status).
			r.With(middleware.RequirePermission(rbac.OrdersView)).Get("/api/admin/shop/orders", adminShopOrderH.List)
			r.With(middleware.RequirePermission(rbac.OrdersManage)).Put("/api/admin/shop/orders/{id}/status", adminShopOrderH.UpdateStatus)

			// Admin storefront profile (GET = view, PUT = upsert).
			r.With(middleware.RequirePermission(rbac.StorefrontView)).Get("/api/admin/storefront", profileH.GetMine)
			r.With(middleware.RequirePermission(rbac.StorefrontManage)).Put("/api/admin/storefront", profileH.Upsert)
		}
	})

	// B2C Shop routes
	if shopEnabled {
		r.Route("/api/shop", func(r chi.Router) {
			r.Get("/shops", shopDirectoryH.List)
			r.Get("/categories", shopCatalogH.ListCategories)
			r.Get("/products", shopCatalogH.ListProducts)
			r.Get("/products/{slug}", shopCatalogH.GetProductBySlug)
			r.Get("/feed", shopCatalogH.Feed)
			r.Get("/banners/active", shopBannerH.ListActive)

			r.Post("/auth/otp/send", shopAuthH.Send)
			r.Post("/auth/otp/verify", shopAuthH.Verify)

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireCustomer(cfg.JWTSecret))

				r.Get("/me", shopCustH.GetMe)
				r.Patch("/me", shopCustH.UpdateMe)

				r.Get("/addresses", shopCustH.ListAddresses)
				r.Post("/addresses", shopCustH.AddAddress)
				r.Patch("/addresses/{id}", shopCustH.UpdateAddress)
				r.Delete("/addresses/{id}", shopCustH.DeleteAddress)
				r.Post("/addresses/{id}/default", shopCustH.SetDefaultAddress)

				r.Get("/cart", shopCartH.Get)
				r.Delete("/cart/items/{product_id}", shopCartH.RemoveItem)

				r.Get("/checkout/summary", shopCheckH.Summary)
				r.Get("/checkout/payment-options", shopCheckH.PaymentOptions)
				r.With(middleware.Idempotency(pool)).Post("/checkout/place", shopCheckH.Place)

				r.With(middleware.Idempotency(pool)).Post("/payments/razorpay/verify", shopPaymentH.Verify)

				r.Get("/orders", shopOrderH.List)
				r.Get("/orders/{id}", shopOrderH.Get)
				r.Post("/orders/{id}/cancel", shopOrderH.Cancel)
			})
		})

		// Per-shop storefront (P4): {shop} slug resolves to the seller org,
		// scoping catalog/feed/banners to that shop. Public, read-only.
		r.Route("/api/shop/s/{shop}", func(r chi.Router) {
			r.Use(middleware.ResolveShop(shopResolve))
			r.Get("/categories", shopCatalogH.ListCategories)
			r.Get("/products", shopCatalogH.ListProducts)
			r.Get("/products/{slug}", shopCatalogH.GetProductBySlug)
			r.Get("/feed", shopCatalogH.Feed)
			r.Get("/banners/active", shopBannerH.ListActive)

			// Cart writes bind the single-shop cart to {shop} (P4 phase 3).
			// Authed; this mount owns /api/shop/s/{shop}/* so the routes must
			// live here rather than in the global authed group (which would be
			// shadowed by this more-specific mount).
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireCustomer(cfg.JWTSecret))
				r.Post("/cart/items", shopCartH.AddItem)
				r.Post("/cart/merge", shopCartH.Merge)
			})
		})
	}

	// otelhttp wraps the whole router so every incoming request gets a
	// server span. Route-template name (e.g. "/api/orders/{id}") shows up
	// as the span name via the operation func.
	return otelhttp.NewHandler(r, "http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
			if rc := req.Context().Value(routeKey{}); rc != nil {
				return req.Method + " " + rc.(string)
			}
			return req.Method + " " + req.URL.Path
		}),
	)
}

type routeKey struct{}

// bannerUploadsHandler serves uploaded banner images with directory-listing
// protection and a one-day public cache header.
func bannerUploadsHandler(uploadDir string) http.Handler {
	fs := http.FileServer(http.Dir(uploadDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject directory listing requests (trailing slash or bare /uploads/).
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.StripPrefix("/uploads/", fs).ServeHTTP(w, r)
	})
}
