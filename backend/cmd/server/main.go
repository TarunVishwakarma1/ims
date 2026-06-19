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
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/crypto"
	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
	"github.com/TarunVishwakarma1/ims/backend/pkg/jobs"
	"github.com/TarunVishwakarma1/ims/backend/pkg/notify"
	"github.com/TarunVishwakarma1/ims/backend/pkg/rbac"
	"github.com/TarunVishwakarma1/ims/backend/pkg/tracing"

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

	// OpenTelemetry tracing. Stdout exporter by default; set OTEL_EXPORTER=otlp
	// and OTEL_ENDPOINT=collector:4317 to ship to Tempo/Jaeger.
	tracerShutdown, err := tracing.Init(ctx, cfg.ENV)
	if err != nil {
		zap.L().Warn("tracing init failed (continuing without traces)", zap.Error(err))
	}
	defer func() {
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = tracerShutdown(shutdownCtx)
	}()

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
	locationRepo := repository.NewLocationRepository(pool)
	marketRepo := repository.NewMarketplaceRepository(pool)
	partnerRepo := repository.NewPartnerRepository(pool)
	returnRepo := repository.NewReturnRepository(pool)
	notificationRepo := repository.NewNotificationRepository(pool)

	// Load permissions cache on startup
	rolePerms, err := roleRepo.LoadRolePermissions(ctx)
	if err != nil {
		zap.L().Fatal("failed to load permissions", zap.Error(err))
	}
	rbac.Cache.Load(rolePerms)
	zap.L().Info("permission cache loaded")

	authRepo := repository.NewAuthRepository(pool)

	// Valkey cache + event bus — both fall back to no-op if unavailable
	cacheClient := cache.MustNew(cfg.ValkeyURL)
	eventBus := events.MustNew(cfg.ValkeyURL)
	defer eventBus.Close()

	// Start background jobs
	jobs.StartReservationExpiry(ctx, marketRepo, 1*time.Minute)
	jobs.StartRefreshTokenCleanup(ctx, authRepo, 1*time.Hour)

	userService := service.NewUserService(userRepo, auditLogRepo)
	categoryService := service.NewCategoryService(categoryRepo, auditLogRepo, cacheClient)
	productService := service.NewProductService(productRepo, inventoryRepo, auditLogRepo, cacheClient, eventBus)
	inventoryService := service.NewInventoryService(inventoryRepo, auditLogRepo, cacheClient, eventBus)
	// Email notifications — log-only fallback if SMTP_HOST not set.
	emailer := notify.MustNew(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFromEmail, cfg.SMTPFromName)
	notifier := notify.NewNotifier(notificationRepo, userRepo, cfg.WebAppURL)
	// Drain queued notifications every 10s. Retries with backoff up to
	// notificationMaxAttempts; failures end up in the DLQ.
	jobs.StartNotificationWorker(ctx, notificationRepo, emailer, 10*time.Second)

	orderService := service.NewOrderService(orderRepo, inventoryRepo, auditLogRepo, marketRepo, eventBus, cacheClient, notifier)
	authService := service.NewAuthService(userRepo, orgRepo, auditLogRepo, authRepo, pool, cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)
	totpService := service.NewTOTPService(userRepo)
	authService.SetTOTPService(totpService)
	authService.SetNotifier(notifier)
	roleService := service.NewRoleService(roleRepo)
	locationService := service.NewLocationService(locationRepo, cacheClient)
	couponRepo := repository.NewCouponRepository(pool)
	couponService := service.NewCouponService(couponRepo)
	marketService := service.NewMarketplaceService(marketRepo, inventoryRepo, orderRepo, productRepo, locationRepo, couponService, cacheClient, eventBus, pool)
	partnerService := service.NewPartnerService(partnerRepo, orgRepo)

	// Encryption for payment payloads / webhook bodies (PII protection).
	encryptor, err := crypto.New(cfg.PayloadEncryptionKey)
	if err != nil {
		zap.L().Fatal("failed to init encryptor", zap.Error(err))
	}
	if encryptor.Enabled() {
		zap.L().Info("payload encryption enabled (AES-256-GCM)")
	}

	paymentRepo := repository.NewPaymentRepository(pool, encryptor)
	webhookRepo := repository.NewWebhookRepository(pool, encryptor)
	paymentService := service.NewPaymentService(paymentRepo, webhookRepo, orderRepo, auditLogRepo,
		userRepo, orgRepo, productRepo, notifier,
		eventBus, cacheClient,
		cfg.RazorpayKeyID, cfg.RazorpayKeySecret, cfg.RazorpayWebhookSecret, cfg.RazorpayWebhookSecretPrev, cfg.RazorpayMockMode)

	// Wire payment → order back-reference so Cancel can auto-refund paid orders.
	orderService.SetPaymentService(paymentService)

	// Daily payment reconciliation — catches drift from missed RazorPay webhooks.
	jobs.StartPaymentReconciliation(ctx, paymentService, 24*time.Hour)

	// Short-cycle stuck-payment reconciliation — every 5 min, asks Razorpay
	// whether a payment was actually captured on orders we still see as
	// 'created'. Marks long-abandoned (>24h) attempts failed so the retry
	// button reactivates on the order.
	jobs.StartStuckPaymentReconciliation(ctx, paymentService, 5*time.Minute, 15)

	returnService := service.NewReturnService(returnRepo, orderRepo, inventoryRepo, auditLogRepo, paymentService, eventBus, notifier)
	notificationService := service.NewNotificationService(notificationRepo)

	userH := handler.NewUserHandler(userService)
	categoryH := handler.NewCategoryHandler(categoryService)
	productH := handler.NewProductHandler(productService)
	inventoryH := handler.NewInventoryHandler(inventoryService)
	orderH := handler.NewOrderHandler(orderService, productService)
	authH := handler.NewAuthHandler(authService, userRepo)
	roleH := handler.NewRoleHandler(roleService)
	locationH := handler.NewLocationHandler(locationService)
	marketH := handler.NewMarketplaceHandler(marketService)
	partnerH := handler.NewPartnerHandler(partnerService)
	returnH := handler.NewReturnHandler(returnService)
	notificationH := handler.NewNotificationHandler(notificationService)
	auditH := handler.NewAuditHandler(auditLogRepo)
	totpH := handler.NewTOTPHandler(totpService, authService, userRepo)
	couponH := handler.NewCouponHandler(couponService)
	mode := "test"
	switch {
	case cfg.RazorpayMockMode:
		mode = "MOCK"
	case cfg.RazorpayLiveMode:
		mode = "LIVE (real money)"
	}
	zap.L().Info("razorpay mode", zap.String("mode", mode), zap.String("key_id", cfg.RazorpayKeyID))
	paymentH := handler.NewPaymentHandler(paymentService, cfg.RazorpayMockMode, cfg.RazorpayLiveMode, cfg.RazorpayKeyID, cfg.RazorpayWebhookSecret, cfg.RazorpayWebhookSecretPrev)
	webhookH := handler.NewWebhookHandler(paymentService)
	eventsH := handler.NewEventsHandler(eventBus)

	router := NewRouter(authH, userH, categoryH, productH, inventoryH, orderH, roleH, locationH, marketH, eventsH, paymentH, webhookH, partnerH, returnH, notificationH, auditH, totpH, couponH, cfg, pool, cacheClient)

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
