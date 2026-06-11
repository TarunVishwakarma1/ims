package main

import (
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/config"
	"github.com/TarunVishwakarma1/ims/backend/internal/handler"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

func NewRouter(
	authH *handler.AuthHandler,
	userH *handler.UserHandler,
	categoryH *handler.CategoryHandler,
	productH *handler.ProductHandler,
	inventoryH *handler.InventoryHandler,
	orderH *handler.OrderHandler,
	cfg *config.Config,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware (applied to ALL routes)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.RateLimiter())
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// Public routes (no auth)
	r.Post("/api/auth/register", userH.CreateUser)
	r.Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/refresh", authH.RefreshToken)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))

		// Users (admin only)
		r.Get("/api/users", userH.ListUsers)
		r.Get("/api/users/{id}", userH.GetUser)
		r.Put("/api/users/{id}", userH.UpdateUser)
		r.Delete("/api/users/{id}", userH.DeleteUser)

		// Categories
		r.Post("/api/categories", categoryH.CreateCategory)
		r.Get("/api/categories", categoryH.ListCategories)
		r.Get("/api/categories/{id}", categoryH.GetCategory)
		r.Put("/api/categories/{id}", categoryH.UpdateCategory)
		r.Delete("/api/categories/{id}", categoryH.DeleteCategory)

		// Products
		r.Post("/api/products", productH.CreateProduct)
		r.Get("/api/products", productH.ListProducts)
		r.Get("/api/products/{id}", productH.GetProduct)
		r.Put("/api/products/{id}", productH.UpdateProduct)
		r.Delete("/api/products/{id}", productH.DeleteProduct)
		r.Get("/api/categories/{category_id}/products", productH.ListByCategory)

		// Inventory
		r.Post("/api/inventory", inventoryH.CreateInventory)
		r.Get("/api/inventory", inventoryH.ListInventory)
		r.Get("/api/inventory/low-stock", inventoryH.ListLowStock)
		r.Get("/api/inventory/product/{product_id}", inventoryH.GetInventoryByProduct)
		r.Put("/api/inventory/{id}", inventoryH.UpdateInventory)

		// Orders
		r.Post("/api/orders", orderH.CreateOrder)
		r.Get("/api/orders", orderH.ListOrders)
		r.Get("/api/orders/{id}", orderH.GetOrder)
		r.Put("/api/orders/{id}/status", orderH.UpdateStatus)
		r.Delete("/api/orders/{id}", orderH.DeleteOrder)
		r.Get("/api/orders/{id}/items", orderH.GetOrderItems)
		r.Get("/api/users/{user_id}/orders", orderH.ListUserOrders)
	})

	return r
}
