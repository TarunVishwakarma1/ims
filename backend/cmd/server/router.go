package main

import (
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/config"
	"github.com/TarunVishwakarma1/ims/backend/internal/handler"
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
	cfg *config.Config,
	pool *pgxpool.Pool,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware (applied to ALL routes)
	r.Use(chiMiddleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.RateLimiter())
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// Public routes (no auth)
	r.Get("/health", handler.HealthCheck(pool))
	r.Post("/api/auth/register", userH.CreateUser)
	r.Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/refresh", authH.RefreshToken)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))

		// Users
		r.With(middleware.RequirePermission(rbac.UsersView)).Get("/api/users", userH.ListUsers)
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
		r.With(middleware.RequirePermission(rbac.RolesManage)).Get("/api/permissions", roleH.ListPermissions)
		r.With(middleware.RequirePermission(rbac.RolesManage)).Put("/api/roles/{id}/permissions", roleH.UpdateRolePermissions)
		r.With(middleware.RequirePermission(rbac.RolesManage)).Post("/api/roles/reload", roleH.ReloadPermissions)
	})

	return r
}
