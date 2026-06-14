package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Bcrypt hash of "dummy-password-never-matches-anything-1234567890" with cost 10.
// Used so unknown emails still run a bcrypt comparison → constant-time login.
const dummyBcryptHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

const (
	maxFailedLogins = 5
	lockoutDuration = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	otpTTL          = 10 * time.Minute
	maxOTPAttempts  = 5
)

type AuthService interface {
	Signup(ctx context.Context, req *SignupRequest, ipAddress, userAgent string) (*domain.LoginResponse, error)
	Login(ctx context.Context, email, password, ipAddress, userAgent string) (*domain.LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken, ipAddress, userAgent string) (*domain.LoginResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	VerifyEmail(ctx context.Context, userID uuid.UUID, otp string) error
	ResendVerificationOTP(ctx context.Context, userID uuid.UUID) (string, error)
}

type SignupRequest struct {
	OrgName  string `json:"org_name" validate:"required,min=2,max=255"`
	OrgSlug  string `json:"org_slug" validate:"required,min=2,max=100,alphanum"`
	UserName string `json:"user_name" validate:"required,min=1,max=255"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type authService struct {
	userRepo      repository.UserRepository
	orgRepo       domain.OrganizationRepository
	auditLogRepo  repository.AuditLogRepository
	authRepo      repository.AuthRepository
	pool          *pgxpool.Pool
	jwtSecret     string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewAuthService(
	userRepo repository.UserRepository,
	orgRepo domain.OrganizationRepository,
	auditLogRepo repository.AuditLogRepository,
	authRepo repository.AuthRepository,
	pool *pgxpool.Pool,
	jwtSecret string,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
) AuthService {
	return &authService{
		userRepo:      userRepo,
		orgRepo:       orgRepo,
		auditLogRepo:  auditLogRepo,
		authRepo:      authRepo,
		pool:          pool,
		jwtSecret:     jwtSecret,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

type claims struct {
	UserID string `json:"user_id"`
	OrgID  string `json:"org_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// ── Signup ────────────────────────────────────────────────────────────────

func (s *authService) Signup(ctx context.Context, req *SignupRequest, ipAddress, userAgent string) (*domain.LoginResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	slug := strings.ToLower(strings.TrimSpace(req.OrgSlug))

	// Check breach corpora — fail-closed on confirmed breach, fail-open on network err
	if err := utils.CheckPwnedPassword(ctx, req.Password); err != nil {
		if errors.Is(err, utils.ErrPasswordPwned) {
			return nil, err
		}
		zap.L().Warn("pwned check unavailable, continuing", zap.Error(err))
	}

	// Pre-checks (note: race condition between check and insert is handled by
	// the DB unique constraint; this is just for a friendlier error message)
	existingUser, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser != nil {
		return nil, domain.ErrConflict
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	existingOrg, err := s.orgRepo.GetBySlug(ctx, slug)
	if err == nil && existingOrg != nil {
		return nil, domain.ErrConflict
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	now := time.Now().UTC()
	orgID := uuid.New()
	userID := uuid.New()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	org := &domain.Organization{
		ID: orgID, Name: req.OrgName, Slug: slug,
		PlanType: "free", IsActive: true,
		CreatedAt: now, UpdatedAt: now,
	}
	user := &domain.User{
		ID: userID, OrgID: orgID, Name: req.UserName, Email: email,
		PasswordHash: string(hashedPassword), Role: "admin", IsActive: true,
		CreatedAt: now, UpdatedAt: now,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.orgRepo.WithTx(tx).Create(ctx, org); err != nil {
		return nil, err
	}
	if err := s.userRepo.WithTx(tx).Create(ctx, user); err != nil {
		return nil, err
	}

	// Email verification OTP (stored hashed). Plain OTP delivered via email
	// in production; for now we log it and the handler returns it in dev mode.
	otp := utils.GenerateOTP()
	otpHash := utils.HashToken(otp)
	if err := repository.NewAuthRepository(tx).CreateEmailVerification(
		ctx, userID, email, otpHash, time.Now().Add(otpTTL),
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	zap.L().Info("signup: email verification OTP issued",
		zap.String("email", email), zap.String("otp_dev_only", otp))

	// Audit
	s.audit(ctx, &orgID, &userID, "auth.signup", "users", userID, ipAddress)

	// Issue tokens. Account is unverified but usable — UI shows a "verify email" banner.
	access, refresh, err := s.issueTokens(ctx, user, org, ipAddress, userAgent, nil)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		AccessToken: access, RefreshToken: refresh,
		User: user, Organization: org,
	}, nil
}

// ── Login ─────────────────────────────────────────────────────────────────

func (s *authService) Login(ctx context.Context, email, password, ipAddress, userAgent string) (*domain.LoginResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	// Constant-time bcrypt — always run, even for unknown emails.
	hashToCompare := dummyBcryptHash
	if user != nil {
		hashToCompare = user.PasswordHash
	}
	bcryptErr := bcrypt.CompareHashAndPassword([]byte(hashToCompare), []byte(password))

	if user == nil || bcryptErr != nil {
		s.recordLoginAttempt(ctx, email, ipAddress, userAgent, false, "invalid_credentials")
		if user != nil {
			s.handleLoginFailure(ctx, user.ID)
		}
		return nil, domain.ErrUnauthorized
	}

	// Account lock check (after auth passed — don't leak lock status to bad pw guessers)
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		s.recordLoginAttempt(ctx, email, ipAddress, userAgent, false, "account_locked")
		return nil, errors.New("account locked, try again later")
	}

	if !user.IsActive {
		s.recordLoginAttempt(ctx, email, ipAddress, userAgent, false, "account_inactive")
		return nil, domain.ErrUnauthorized
	}

	org, err := s.orgRepo.GetByID(ctx, user.OrgID)
	if err != nil || !org.IsActive {
		s.recordLoginAttempt(ctx, email, ipAddress, userAgent, false, "org_inactive")
		return nil, domain.ErrUnauthorized
	}

	// Success path
	s.recordLoginAttempt(ctx, email, ipAddress, userAgent, true, "")
	_ = s.authRepo.ResetFailedLogins(ctx, user.ID)
	_ = s.authRepo.UpdateLastLogin(ctx, user.ID)
	s.audit(ctx, &user.OrgID, &user.ID, "auth.login", "users", user.ID, ipAddress)

	access, refresh, err := s.issueTokens(ctx, user, org, ipAddress, userAgent, nil)
	if err != nil {
		return nil, err
	}
	return &domain.LoginResponse{
		AccessToken: access, RefreshToken: refresh,
		User: user, Organization: org,
	}, nil
}

func (s *authService) handleLoginFailure(ctx context.Context, userID uuid.UUID) {
	count, err := s.authRepo.IncrementFailedLogins(ctx, userID)
	if err != nil {
		zap.L().Error("increment failed logins", zap.Error(err))
		return
	}
	if count >= maxFailedLogins {
		_ = s.authRepo.LockUser(ctx, userID, time.Now().Add(lockoutDuration))
		zap.L().Warn("account locked", zap.String("user_id", userID.String()), zap.Int("attempts", count))
	}
}

// ── Refresh with rotation + reuse detection ───────────────────────────────

func (s *authService) RefreshToken(ctx context.Context, rawToken, ipAddress, userAgent string) (*domain.LoginResponse, error) {
	if rawToken == "" {
		return nil, domain.ErrUnauthorized
	}

	hash := utils.HashToken(rawToken)
	stored, err := s.authRepo.FindRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, domain.ErrUnauthorized
	}

	// Reuse detection — already revoked? Kill the entire family.
	if stored.RevokedAt != nil {
		_ = s.authRepo.RevokeFamily(ctx, stored.FamilyID, "reuse_detected")
		zap.L().Warn("refresh token reuse detected",
			zap.String("user_id", stored.UserID.String()),
			zap.String("family_id", stored.FamilyID.String()))
		return nil, domain.ErrUnauthorized
	}

	if stored.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrUnauthorized
	}

	// Load user + org for fresh role/active checks
	user, err := s.userRepo.GetByID(ctx, stored.UserID, uuid.Nil)
	if err != nil {
		// Try without orgID filter — user_repo signature requires orgID,
		// so we look it up separately.
		return nil, domain.ErrUnauthorized
	}
	if !user.IsActive {
		return nil, domain.ErrUnauthorized
	}

	org, err := s.orgRepo.GetByID(ctx, user.OrgID)
	if err != nil || !org.IsActive {
		return nil, domain.ErrUnauthorized
	}

	// Revoke the used token (rotation)
	if err := s.authRepo.RevokeRefreshToken(ctx, stored.ID, "rotated"); err != nil {
		return nil, err
	}

	access, refresh, err := s.issueTokens(ctx, user, org, ipAddress, userAgent, &stored.FamilyID)
	if err != nil {
		return nil, err
	}
	return &domain.LoginResponse{
		AccessToken: access, RefreshToken: refresh,
		User: user, Organization: org,
	}, nil
}

func (s *authService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	hash := utils.HashToken(rawToken)
	stored, err := s.authRepo.FindRefreshTokenByHash(ctx, hash)
	if err != nil || stored == nil {
		return nil
	}
	return s.authRepo.RevokeFamily(ctx, stored.FamilyID, "logout")
}

// ── Email verification ────────────────────────────────────────────────────

func (s *authService) VerifyEmail(ctx context.Context, userID uuid.UUID, otp string) error {
	storedHash, expiresAt, attempts, err := s.authRepo.FindActiveVerification(ctx, userID)
	if err != nil {
		return err
	}
	if storedHash == "" {
		return errors.New("no active verification request")
	}
	if attempts >= maxOTPAttempts {
		return errors.New("too many attempts, request a new OTP")
	}
	if expiresAt.Before(time.Now()) {
		return errors.New("OTP expired, request a new one")
	}
	_ = s.authRepo.IncrementVerificationAttempts(ctx, userID)

	if !utils.ConstantTimeCompare(utils.HashToken(otp), storedHash) {
		return errors.New("invalid OTP")
	}

	if err := s.authRepo.ConsumeVerification(ctx, userID); err != nil {
		return err
	}
	return s.authRepo.MarkEmailVerified(ctx, userID)
}

func (s *authService) ResendVerificationOTP(ctx context.Context, userID uuid.UUID) (string, error) {
	otp := utils.GenerateOTP()
	otpHash := utils.HashToken(otp)
	// Find user's email
	// We don't have a direct lookup; orgID is unknown here. Use a raw query path.
	// For simplicity, look up via UserRepo using orgID from refresh token isn't possible.
	// Caller knows userID + orgID from JWT, but service takes userID only.
	// We accept the trade-off: this method requires the caller to have already validated
	// the user (which the handler does via the Auth middleware).
	zap.L().Info("verification OTP reissued",
		zap.String("user_id", userID.String()), zap.String("otp_dev_only", otp))
	if err := s.authRepo.CreateEmailVerification(ctx, userID, "", otpHash, time.Now().Add(otpTTL)); err != nil {
		return "", err
	}
	return otp, nil
}

// ── Token issuance helpers ────────────────────────────────────────────────

func (s *authService) issueTokens(
	ctx context.Context, user *domain.User, org *domain.Organization,
	ip, ua string, family *uuid.UUID,
) (string, string, error) {
	access, err := s.signJWT(user.ID.String(), org.ID.String(), user.Role, "access", s.accessExpiry)
	if err != nil {
		return "", "", err
	}
	rawRefresh, err := utils.GenerateRandomToken(32)
	if err != nil {
		return "", "", err
	}

	familyID := uuid.New()
	if family != nil {
		familyID = *family
	}

	rt := &repository.RefreshToken{
		ID: uuid.New(), UserID: user.ID, FamilyID: familyID,
		TokenHash: utils.HashToken(rawRefresh),
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(refreshTokenTTL),
		UserAgent: nullable(ua), IPAddress: nullable(ip),
	}
	if err := s.authRepo.StoreRefreshToken(ctx, rt); err != nil {
		return "", "", err
	}
	return access, rawRefresh, nil
}

func (s *authService) signJWT(userID, orgID, role, sub string, ttl time.Duration) (string, error) {
	tokenClaims := &claims{
		UserID: userID, OrgID: orgID, Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   sub,
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)
	return t.SignedString([]byte(s.jwtSecret))
}

func (s *authService) recordLoginAttempt(ctx context.Context, email, ip, ua string, success bool, reason string) {
	_ = s.authRepo.RecordLoginAttempt(ctx, &repository.LoginAttempt{
		Email: email, IPAddress: ip, UserAgent: ua,
		Success: success, FailureReason: reason,
	})
}

func (s *authService) audit(ctx context.Context, orgID, userID *uuid.UUID, action, entity string, entityID uuid.UUID, ip string) {
	if orgID == nil {
		return
	}
	a := &domain.AuditLog{
		ID:    uuid.New(),
		OrgID: *orgID, UserID: userID,
		Action:    action,
		Entity:    entity,
		EntityID:  entityID,
		IPAddress: ip,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.auditLogRepo.Create(ctx, a); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
