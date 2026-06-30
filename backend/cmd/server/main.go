package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/config"
	"github.com/TarunVishwakarma1/ims/backend/internal/devseed"
	"github.com/TarunVishwakarma1/ims/backend/internal/handler"
	shophandler "github.com/TarunVishwakarma1/ims/backend/internal/handler/shop"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	shopsvc "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
	"github.com/TarunVishwakarma1/ims/backend/migrations"
	"github.com/TarunVishwakarma1/ims/backend/pkg/cache"
	"github.com/TarunVishwakarma1/ims/backend/pkg/calendar"
	"github.com/TarunVishwakarma1/ims/backend/pkg/crypto"
	"github.com/TarunVishwakarma1/ims/backend/pkg/events"
	"github.com/TarunVishwakarma1/ims/backend/pkg/jobs"
	"github.com/TarunVishwakarma1/ims/backend/pkg/logger"
	"github.com/TarunVishwakarma1/ims/backend/pkg/notify"
	"github.com/TarunVishwakarma1/ims/backend/pkg/rbac"
	"github.com/TarunVishwakarma1/ims/backend/pkg/sms"
	"github.com/TarunVishwakarma1/ims/backend/pkg/storage"
	"github.com/TarunVishwakarma1/ims/backend/pkg/tracing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("cannot create UPLOAD_DIR %q: %v", cfg.UploadDir, err)
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

	// B2C Shop services and handlers
	var (
		shopAuthH    *shophandler.AuthHandler
		shopCustH    *shophandler.CustomerHandler
		shopCartH    *shophandler.CartHandler
		shopCheckH   *shophandler.CheckoutHandler
		shopCatalogH *shophandler.CatalogHandler
		shopBannerH  *shophandler.BannerHandler
		shopOrderH      *shophandler.OrderHandler
		adminShopOrderH *shophandler.AdminOrderHandler
		shopDirectoryH  *shophandler.DirectoryHandler
		shopResolve     func(context.Context, string) (uuid.UUID, error)
		shopPaymentH    *shophandler.PaymentHandler
		adminBannerH *handler.AdminBannerHandler
	)
	if cfg.ShopEnabled {
		shopOrgID, err := resolveShopOrg(context.Background(), pool, cfg.ShopOrgID, cfg.ShopOrgSlug, cfg.ShopOrgName)
		if err != nil {
			zap.L().Fatal("resolve shop org", zap.Error(err))
		}
		zap.L().Info("shop org resolved",
			zap.String("id", shopOrgID.String()),
			zap.String("slug", cfg.ShopOrgSlug),
			zap.String("name", cfg.ShopOrgName))

		var smsSender sms.Sender
		if cfg.MSG91AuthKey != "" {
			smsSender = sms.NewMSG91(cfg.MSG91AuthKey, cfg.MSG91TemplateID, cfg.MSG91SenderID, &http.Client{Timeout: 10 * time.Second})
		} else {
			smsSender = &sms.MockSender{}
			zap.L().Warn("SHOP_ENABLED=true but MSG91_AUTH_KEY empty — using MockSender (dev only)")
		}

		customerRepo := repository.NewCustomerRepository(pool)
		otpRepo := repository.NewOTPRepository(pool)
		addrRepo := repository.NewCustomerAddressRepository(pool)
		cartRepo := repository.NewCartRepository(pool)

		otpSvc := shopsvc.NewOTPService(otpRepo, customerRepo, smsSender, cfg.JWTSecret)
		custSvc := shopsvc.NewCustomerService(customerRepo, addrRepo)
		cartSvc := shopsvc.NewCartService(cartRepo, pool, shopOrgID)
		checkSvc := shopsvc.NewCheckoutService(pool, shopOrgID, cartRepo, addrRepo, paymentService, orderRepo, couponService, cfg.RazorpayKeyID, cfg.ShopCODMinPaise, cfg.ShopCODMaxPaise, cfg.ShopPlatformPaise, cfg.ShopShippingPaise, cfg.ShopFreeShipThreshPaise)

		// Customer-facing order emails (queued for the notification worker).
		shopNotifier := shopsvc.NewShopNotifier(notificationRepo, customerRepo, orderRepo, cfg.WebAppURL)
		// Razorpay webhook → bus → customer payment/refund emails.
		shopsvc.StartPaymentEventListener(ctx, eventBus, shopOrgID, shopNotifier)

		shopAuthH = shophandler.NewAuthHandler(otpSvc)
		shopCustH = shophandler.NewCustomerHandler(custSvc)
		shopCartH = shophandler.NewCartHandler(cartSvc)
		shopCheckH = shophandler.NewCheckoutHandler(checkSvc, shopNotifier)

		catalogSvc := shopsvc.NewCatalogService(pool, cacheClient, shopOrgID)
		feedSvc := shopsvc.NewFeedService(pool, cacheClient, shopOrgID)
		shopCatalogH = shophandler.NewCatalogHandler(catalogSvc, feedSvc)

		if cfg.SeedDevData {
			seedCtx := context.Background()
			if err := devseed.Run(seedCtx, pool, shopOrgID); err != nil {
				zap.L().Error("dev seed failed", zap.Error(err))
			} else {
				zap.L().Info("dev seed applied")
				if err := catalogSvc.InvalidateCategories(seedCtx); err != nil {
					zap.L().Warn("dev seed: invalidate categories cache", zap.Error(err))
				}
				if err := catalogSvc.InvalidateProductList(seedCtx); err != nil {
					zap.L().Warn("dev seed: invalidate product list cache", zap.Error(err))
				}
			}
		}

		bannerRepo := repository.NewBannerRepository(pool)
		bannerSvc := shopsvc.NewBannerService(bannerRepo, cacheClient, shopOrgID)
		diskStore := storage.NewDiskStorage(cfg.UploadDir, "/uploads")
		adminBannerH = handler.NewAdminBannerHandler(bannerSvc, diskStore, cfg.BannerImageMaxBytes)
		shopBannerH = shophandler.NewBannerHandler(bannerSvc)

		orderSvcShop := shopsvc.NewShopOrderService(pool, orderRepo, paymentService, shopOrgID)
		shopOrderH = shophandler.NewOrderHandler(orderSvcShop, shopNotifier)
		adminShopOrderH = shophandler.NewAdminOrderHandler(orderSvcShop, shopNotifier)
		shopDirectorySvc := shopsvc.NewShopDirectoryService(pool)
		shopDirectoryH = shophandler.NewDirectoryHandler(shopDirectorySvc)
		shopResolve = shopDirectorySvc.OrgBySlug

		shopPaymentSvc := shopsvc.NewShopPaymentService(
			pool, shopOrgID,
			orderRepo,
			paymentRepo,
			cfg.RazorpayKeySecret,
			cfg.RazorpayMockMode,
		)
		shopPaymentH = shophandler.NewPaymentHandler(shopPaymentSvc, shopNotifier)

		if cfg.BannerSeedEnabled {
			go func() {
				stop := jobs.StartBannerSeed(ctx, pool, cacheClient, shopOrgID,
					calendar.Festivals, cfg.BannerSeedInterval)
				<-ctx.Done()
				stop()
			}()
		}

		// Background popularity recompute. 30-min interval; stops with ctx.
		go func() {
			stop := jobs.StartPopularityRecompute(ctx, cacheClient, pool, shopOrgID, 30*time.Minute)
			<-ctx.Done()
			stop()
		}()
	}

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

	router := NewRouter(authH, userH, categoryH, productH, inventoryH, orderH, roleH, locationH, marketH, eventsH, paymentH, webhookH, partnerH, returnH, notificationH, auditH, totpH, couponH, cfg, pool, cacheClient, cfg.ShopEnabled, shopAuthH, shopCustH, shopCartH, shopCheckH, shopCatalogH, adminBannerH, shopBannerH, shopOrderH, shopPaymentH, adminShopOrderH, shopDirectoryH, shopResolve, cfg.UploadDir)

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

// resolveShopOrg returns the UUID of the B2C shop organization.
//
// Resolution order:
//  1. If explicitOverride (SHOP_ORG_ID env) is set, parse and return it.
//     Validates that a row with this id actually exists.
//  2. Else SELECT id FROM organizations WHERE slug=$slug.
//  3. Else INSERT a new row with (slug, name, plan_type='enterprise').
//
// The hybrid migration (000020_seed_shop_org) seeds a canonical row on
// fresh DBs; this function handles existing DBs that pre-date the migration
// and self-heals when an operator drops the row by mistake.
func resolveShopOrg(ctx context.Context, pool *pgxpool.Pool, explicitID, slug, name string) (uuid.UUID, error) {
	if explicitID != "" {
		id, err := uuid.Parse(explicitID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid SHOP_ORG_ID %q: %w", explicitID, err)
		}
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM organizations WHERE id=$1)`, id,
		).Scan(&exists); err != nil {
			return uuid.Nil, fmt.Errorf("verify SHOP_ORG_ID: %w", err)
		}
		if !exists {
			return uuid.Nil, fmt.Errorf("SHOP_ORG_ID %s not present in organizations", id)
		}
		return id, nil
	}

	if slug == "" {
		return uuid.Nil, fmt.Errorf("SHOP_ORG_SLUG empty and SHOP_ORG_ID unset")
	}

	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM organizations WHERE slug=$1`, slug,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("lookup shop org by slug %q: %w", slug, err)
	}

	// Self-heal: row missing (fresh DB pre-migration, or operator deletion).
	displayName := name
	if displayName == "" {
		displayName = slug
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug, plan_type, is_active)
		VALUES ($1, $2, 'enterprise', true)
		RETURNING id
	`, displayName, slug).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("bootstrap shop org %q: %w", slug, err)
	}
	zap.L().Info("bootstrapped shop org",
		zap.String("id", id.String()),
		zap.String("slug", slug),
		zap.String("name", displayName))
	return id, nil
}
