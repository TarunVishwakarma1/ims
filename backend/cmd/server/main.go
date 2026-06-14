package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/config"
	"github.com/TarunVishwakarma1/ims/backend/internal/handler"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/migrations"
	"github.com/TarunVishwakarma1/ims/backend/pkg/logger"
	"github.com/TarunVishwakarma1/ims/backend/pkg/rbac"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
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

	appLogger, err := logger.New(cfg.ENV)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer appLogger.Sync()
	zap.ReplaceGlobals(appLogger)

	// Run Migrations
	d, err := iofs.New(migrations.FS, ".")
	if err != nil {
		zap.L().Fatal("failed to initialize migrations iofs", zap.Error(err))
	}

	dbURL := strings.ReplaceAll(cfg.DatabaseURL, "postgres://", "pgx5://")
	dbURL = strings.ReplaceAll(dbURL, "postgresql://", "pgx5://")

	m, err := migrate.NewWithSourceInstance("iofs", d, dbURL)
	if err != nil {
		zap.L().Fatal("failed to create migrate instance", zap.Error(err))
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			zap.L().Warn("migrate source close error", zap.Error(srcErr))
		}
		if dbErr != nil {
			zap.L().Warn("migrate db close error", zap.Error(dbErr))
		}
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		zap.L().Fatal("migration failed", zap.Error(err))
	}
	zap.L().Info("migrations applied successfully")

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		zap.L().Fatal("failed to create DB pool", zap.Error(err))
	}
	defer pool.Close()

	auditLogRepo := repository.NewAuditLogRepository(pool)
	orgRepo := repository.NewOrganizationRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	productRepo := repository.NewProductRepository(pool)
	inventoryRepo := repository.NewInventoryRepository(pool)
	orderRepo := repository.NewOrderRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)

	// Load permissions cache on startup
	rolePerms, err := roleRepo.LoadRolePermissions(ctx)
	if err != nil {
		zap.L().Fatal("failed to load permissions", zap.Error(err))
	}
	rbac.Cache.Load(rolePerms)
	zap.L().Info("permission cache loaded")

	userService := service.NewUserService(userRepo, auditLogRepo)
	categoryService := service.NewCategoryService(categoryRepo, auditLogRepo)
	productService := service.NewProductService(productRepo, inventoryRepo, auditLogRepo)
	inventoryService := service.NewInventoryService(inventoryRepo, auditLogRepo)
	orderService := service.NewOrderService(orderRepo, inventoryRepo, auditLogRepo)
	authService := service.NewAuthService(userRepo, orgRepo, auditLogRepo, pool, cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)
	roleService := service.NewRoleService(roleRepo)

	userH := handler.NewUserHandler(userService)
	categoryH := handler.NewCategoryHandler(categoryService)
	productH := handler.NewProductHandler(productService)
	inventoryH := handler.NewInventoryHandler(inventoryService)
	orderH := handler.NewOrderHandler(orderService, productService)
	authH := handler.NewAuthHandler(authService)
	roleH := handler.NewRoleHandler(roleService)

	router := NewRouter(authH, userH, categoryH, productH, inventoryH, orderH, roleH, cfg, pool)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		zap.L().Info("Starting server", zap.String("port", cfg.Port), zap.String("env", cfg.ENV))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("failed to listen and serve", zap.Error(err))
		}
	}()

	<-stop
	zap.L().Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		zap.L().Fatal("server forced shutdown", zap.Error(err))
	}

	zap.L().Info("Server exiting gracefully")
}
