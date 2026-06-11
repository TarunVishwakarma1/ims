package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/config"
	"github.com/TarunVishwakarma1/ims/backend/internal/handler"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to create DB pool: %v", err)
	}
	defer pool.Close()

	auditLogRepo := repository.NewAuditLogRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	productRepo := repository.NewProductRepository(pool)
	inventoryRepo := repository.NewInventoryRepository(pool)
	orderRepo := repository.NewOrderRepository(pool)

	userService := service.NewUserService(userRepo, auditLogRepo)
	categoryService := service.NewCategoryService(categoryRepo, auditLogRepo)
	productService := service.NewProductService(productRepo, auditLogRepo)
	inventoryService := service.NewInventoryService(inventoryRepo, auditLogRepo)
	orderService := service.NewOrderService(orderRepo, inventoryRepo, auditLogRepo)
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)

	userH := handler.NewUserHandler(userService)
	categoryH := handler.NewCategoryHandler(categoryService)
	productH := handler.NewProductHandler(productService)
	inventoryH := handler.NewInventoryHandler(inventoryService)
	orderH := handler.NewOrderHandler(orderService, productService)
	authH := handler.NewAuthHandler(authService)

	router := NewRouter(authH, userH, categoryH, productH, inventoryH, orderH, cfg)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Starting server on port %s in %s mode...", cfg.Port, cfg.ENV)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to listen and serve: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced shutdown: %v", err)
	}

	log.Println("Server exiting gracefully")
}
