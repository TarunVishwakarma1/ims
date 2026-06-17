package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/pkg/notify"
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

// slugPattern matches DNS-safe org slugs: lowercase letters, digits, hyphens.
// No leading/trailing/consecutive hyphens.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type AuthService interface {
	Signup(ctx context.Context, req *SignupRequest, ipAddress, userAgent string) (*domain.LoginResponse, error)
	Login(ctx context.Context, email, password, ipAddress, userAgent string) (*domain.LoginResponse, error)
	VerifyTOTPLogin(ctx context.Context, pendingToken, code, ipAddress, userAgent string) (*domain.LoginResponse, error)
	// ResendLoginEmailOTP re-issues a fresh email OTP for an in-progress
	// login. pendingToken carries the user id.
	ResendLoginEmailOTP(ctx context.Context, pendingToken, ip string) error
	RefreshToken(ctx context.Context, refreshToken, ipAddress, userAgent string) (*domain.LoginResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	VerifyEmail(ctx context.Context, userID uuid.UUID, otp string) error
	ResendVerificationOTP(ctx context.Context, userID uuid.UUID) (string, error)
	SetTOTPService(t TOTPService)
	SetNotifier(n notify.Notifier)

	// RequestPasswordReset issues a one-hour reset token for the account
	// owning `email`. Returns nil regardless of whether the account exists
	// — defends against email enumeration.
	RequestPasswordReset(ctx context.Context, email, ip, ua string) error

	// ConfirmPasswordReset consumes a reset token and sets a new bcrypt
	// hash. Rejects expired / consumed / unknown tokens with ErrUnauthorized.
	ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error
}

type SignupRequest struct {
	OrgName  string `json:"org_name" validate:"required,min=2,max=255"`
	OrgSlug  string `json:"org_slug" validate:"required,min=2,max=100"`
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
	totp          TOTPService // optional; wired post-construction
	notifier      notify.Notifier
}

// SetNotifier wires the email notifier post-construction so the existing
// constructor signature stays unchanged.
func (s *authService) SetNotifier(n notify.Notifier) {
	s.notifier = n
}

// SetTOTPService wires the TOTP validator. Decoupled from constructor to
// keep the existing call site untouched and avoid a chicken-and-egg
// problem if TOTPService ever depends on auth.
func (s *authService) SetTOTPService(t TOTPService) {
	s.totp = t
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

	// Slug must be URL-friendly: lowercase letters, digits, and hyphens.
	if !slugPattern.MatchString(slug) {
		return nil, errors.New("organization slug must contain only lowercase letters, numbers, and hyphens")
	}

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
		zap.String("email", email))
	// Send the OTP email. Notifier is nil-safe if email is disabled.
	if s.notifier != nil {
		s.notifier.EmailVerificationOTP(ctx, email, otp, int(otpTTL.Minutes()))
	}

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

	// Password is correct. If the user has any 2FA factor on, halt here
	// and return a short-lived "pending" JWT. Client then calls
	// VerifyTOTPLogin (despite the name, accepts TOTP or email OTP) with
	// the rotating code to receive real tokens.
	if user.TOTPEnabled || user.EmailTwoFAEnabled {
		pending, err := s.signJWT(user.ID.String(), user.OrgID.String(), user.Role, "2fa_pending", 5*time.Minute)
		if err != nil {
			return nil, err
		}
		// Email-only factor: mint + email an OTP immediately so the user
		// can finish login on the very next request without an extra round
		// trip. If TOTP is also enabled, we prefer TOTP and don't email.
		method := "totp"
		if user.EmailTwoFAEnabled && !user.TOTPEnabled {
			method = "email"
			s.issueLoginEmailOTP(ctx, user, ipAddress)
		}
		return &domain.LoginResponse{
			RequireTOTP:  true,
			PendingToken: pending,
			TwoFAMethod:  method,
		}, nil
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

const maxLoginOTPAttempts = 5

// validateLoginEmailOTP checks the supplied code against the active OTP
// for the user. Bumps attempts on failure; consumes on success. Returns
// ErrUnauthorized for any failure mode to avoid leaking which code worked.
func (s *authService) validateLoginEmailOTP(ctx context.Context, userID uuid.UUID, code string) error {
	hash, attempts, err := s.authRepo.FindActiveLoginOTP(ctx, userID)
	if err != nil {
		return err
	}
	if hash == "" {
		return domain.ErrUnauthorized
	}
	if attempts >= maxLoginOTPAttempts {
		_ = s.authRepo.ConsumeLoginOTP(ctx, userID)
		return domain.ErrUnauthorized
	}
	if hash != utils.HashToken(code) {
		_ = s.authRepo.IncrementLoginOTPAttempts(ctx, userID)
		return domain.ErrUnauthorized
	}
	return s.authRepo.ConsumeLoginOTP(ctx, userID)
}

// issueLoginEmailOTP generates a 6-digit code, stores its hash with a
// 10-minute TTL, and dispatches the code to the user's email. Failures
// are logged so login can still continue if the user resends.
func (s *authService) issueLoginEmailOTP(ctx context.Context, user *domain.User, ip string) {
	otp := utils.GenerateOTP()
	otpHash := utils.HashToken(otp)
	if err := s.authRepo.CreateLoginOTP(ctx, user.ID, otpHash, time.Now().Add(otpTTL), ip); err != nil {
		zap.L().Warn("create login OTP failed", zap.Error(err))
		return
	}
	if s.notifier != nil {
		s.notifier.EmailVerificationOTP(ctx, user.Email, otp, int(otpTTL.Minutes()))
	}
	zap.L().Info("login OTP issued", zap.String("user_id", user.ID.String()))
}

// VerifyTOTPLogin completes the two-step login: verifies the pending JWT,
// then validates the TOTP / backup code. On success, issues real tokens.
func (s *authService) VerifyTOTPLogin(ctx context.Context, pendingToken, code, ipAddress, userAgent string) (*domain.LoginResponse, error) {
	if pendingToken == "" || code == "" {
		return nil, domain.ErrUnauthorized
	}
	c := &claims{}
	tok, err := jwt.ParseWithClaims(pendingToken, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil || !tok.Valid || c.Subject != "2fa_pending" {
		return nil, domain.ErrUnauthorized
	}
	userID, err := uuid.Parse(c.UserID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	user, err := s.userRepo.GetByID(ctx, userID, uuid.Nil)
	if err != nil {
		return nil, err
	}
	org, err := s.orgRepo.GetByID(ctx, user.OrgID)
	if err != nil {
		return nil, err
	}
	// Pick a validator. TOTP wins when both factors are on (rotating code
	// is stronger than email). Email path falls back when only it is on.
	switch {
	case user.TOTPEnabled:
		if s.totp == nil {
			return nil, errors.New("2FA service not configured")
		}
		if err := s.totp.Validate(ctx, user, code); err != nil {
			s.recordLoginAttempt(ctx, user.Email, ipAddress, userAgent, false, "invalid_2fa")
			return nil, domain.ErrUnauthorized
		}
	case user.EmailTwoFAEnabled:
		if err := s.validateLoginEmailOTP(ctx, user.ID, code); err != nil {
			s.recordLoginAttempt(ctx, user.Email, ipAddress, userAgent, false, "invalid_email_2fa")
			return nil, domain.ErrUnauthorized
		}
	default:
		// User flipped a flag off mid-login. Reject — caller should retry login.
		return nil, domain.ErrUnauthorized
	}
	s.recordLoginAttempt(ctx, user.Email, ipAddress, userAgent, true, "")
	_ = s.authRepo.ResetFailedLogins(ctx, user.ID)
	_ = s.authRepo.UpdateLastLogin(ctx, user.ID)
	s.audit(ctx, &user.OrgID, &user.ID, "auth.login.2fa", "users", user.ID, ipAddress)

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
	// Look up user so we have an email to send to. uuid.Nil for orgID does a
	// cross-org lookup (refresh-token path uses the same trick).
	user, err := s.userRepo.GetByID(ctx, userID, uuid.Nil)
	if err != nil {
		return "", err
	}
	if err := s.authRepo.CreateEmailVerification(ctx, userID, user.Email, otpHash, time.Now().Add(otpTTL)); err != nil {
		return "", err
	}
	zap.L().Info("verification OTP reissued",
		zap.String("user_id", userID.String()))
	if s.notifier != nil {
		s.notifier.EmailVerificationOTP(ctx, user.Email, otp, int(otpTTL.Minutes()))
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

// ── Password reset ────────────────────────────────────────────────────────

const passwordResetTTL = time.Hour

// RequestPasswordReset is intentionally silent on whether the email maps
// to a real account. UI shows the same success message either way so
// attackers can't enumerate the user table.
func (s *authService) RequestPasswordReset(ctx context.Context, email, ip, ua string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		zap.L().Info("password reset request — no matching user (silenced)",
			zap.String("email", email))
		return nil
	}

	rawToken, err := utils.GenerateRandomToken(32) // 256 bits of entropy
	if err != nil {
		return err
	}
	tokenHash := utils.HashToken(rawToken)
	expiresAt := time.Now().Add(passwordResetTTL).UTC()
	if err := s.authRepo.CreatePasswordReset(ctx, user.ID, tokenHash, expiresAt, ip, ua); err != nil {
		return err
	}
	s.audit(ctx, &user.OrgID, &user.ID, "auth.password_reset.requested", "users", user.ID, ip)
	if s.notifier != nil {
		s.notifier.PasswordReset(ctx, user.Email, rawToken, int(passwordResetTTL.Minutes()))
	}
	return nil
}

// ConfirmPasswordReset validates the raw token, swaps the user's password,
// consumes the token, and revokes every refresh-token family so existing
// sessions can't continue with the old password.
func (s *authService) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if rawToken == "" || len(newPassword) < 8 {
		return domain.ErrUnauthorized
	}
	tokenHash := utils.HashToken(rawToken)
	userID, err := s.authRepo.FindActivePasswordResetByHash(ctx, tokenHash)
	if err != nil {
		return domain.ErrUnauthorized
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.authRepo.SetPasswordHash(ctx, userID, string(hashed)); err != nil {
		return err
	}
	if err := s.authRepo.ConsumePasswordReset(ctx, tokenHash); err != nil {
		zap.L().Warn("consume reset token failed", zap.Error(err))
	}
	// Kill every active refresh token for this user — old sessions can't
	// keep refreshing into new access tokens after the password changes.
	if err := s.authRepo.RevokeAllForUser(ctx, userID, "password_reset"); err != nil {
		zap.L().Warn("revoke refresh tokens after password reset failed", zap.Error(err))
	}
	s.audit(ctx, nil, &userID, "auth.password_reset.confirmed", "users", userID, "")
	return nil
}

// ResendLoginEmailOTP validates the pending JWT and re-sends an email OTP
// for the user it references. Only meaningful for email-2FA users; TOTP
// users don't need it. Silently no-ops if the user has TOTP only.
func (s *authService) ResendLoginEmailOTP(ctx context.Context, pendingToken, ip string) error {
	if pendingToken == "" {
		return domain.ErrUnauthorized
	}
	c := &claims{}
	tok, err := jwt.ParseWithClaims(pendingToken, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil || !tok.Valid || c.Subject != "2fa_pending" {
		return domain.ErrUnauthorized
	}
	userID, err := uuid.Parse(c.UserID)
	if err != nil {
		return domain.ErrUnauthorized
	}
	user, err := s.userRepo.GetByID(ctx, userID, uuid.Nil)
	if err != nil {
		return err
	}
	if !user.EmailTwoFAEnabled {
		return nil // TOTP-only user; nothing to resend
	}
	s.issueLoginEmailOTP(ctx, user, ip)
	return nil
}
