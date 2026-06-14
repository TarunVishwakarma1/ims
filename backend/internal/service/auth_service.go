package service

import (
	"context"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Signup(ctx context.Context, req *SignupRequest, ipAddress string) (*domain.LoginResponse, error)
	Login(ctx context.Context, email, password, ipAddress string) (*domain.LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*domain.LoginResponse, error)
}

type SignupRequest struct {
	OrgName  string `json:"org_name" validate:"required"`
	OrgSlug  string `json:"org_slug" validate:"required"`
	UserName string `json:"user_name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type authService struct {
	userRepo      repository.UserRepository
	orgRepo       domain.OrganizationRepository
	auditLogRepo  repository.AuditLogRepository
	pool          *pgxpool.Pool
	jwtSecret     string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewAuthService(
	userRepo repository.UserRepository,
	orgRepo domain.OrganizationRepository,
	auditLogRepo repository.AuditLogRepository,
	pool *pgxpool.Pool,
	jwtSecret string,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
) AuthService {
	return &authService{
		userRepo:      userRepo,
		orgRepo:       orgRepo,
		auditLogRepo:  auditLogRepo,
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

func (s *authService) Signup(ctx context.Context, req *SignupRequest, ipAddress string) (*domain.LoginResponse, error) {
	// Check if email already exists globally
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, domain.ErrConflict
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	// Check if org slug already exists
	existingOrg, err := s.orgRepo.GetBySlug(ctx, req.OrgSlug)
	if err == nil && existingOrg != nil {
		return nil, domain.ErrConflict
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	now := time.Now().UTC()
	orgID := uuid.New()
	userID := uuid.New()

	org := &domain.Organization{
		ID:        orgID,
		Name:      req.OrgName,
		Slug:      req.OrgSlug,
		PlanType:  "free",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:           userID,
		OrgID:        orgID,
		Name:         req.UserName,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Role:         "admin", // First user is always admin
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txOrgRepo := s.orgRepo.WithTx(tx)
	txUserRepo := s.userRepo.WithTx(tx)

	if err := txOrgRepo.Create(ctx, org); err != nil {
		return nil, err
	}

	if err := txUserRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	audit := &domain.AuditLog{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    &user.ID,
		Action:    "org.signup",
		Entity:    "organizations",
		EntityID:  orgID,
		IPAddress: ipAddress,
		CreatedAt: now,
	}
	if err := s.auditLogRepo.Create(ctx, audit); err != nil {
		zap.L().Error("audit log failed", zap.Error(err))
	}

	// Generate tokens
	accessToken, err := s.generateToken(user.ID.String(), org.ID.String(), string(user.Role), "access", s.accessExpiry)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateToken(user.ID.String(), org.ID.String(), string(user.Role), "refresh", s.refreshExpiry)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
		Organization: org,
	}, nil
}

func (s *authService) Login(ctx context.Context, email, password, ipAddress string) (*domain.LoginResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	if !user.IsActive {
		return nil, domain.ErrUnauthorized
	}

	org, err := s.orgRepo.GetByID(ctx, user.OrgID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	if !org.IsActive {
		return nil, domain.ErrUnauthorized
	}

	accessToken, err := s.generateToken(user.ID.String(), org.ID.String(), string(user.Role), "access", s.accessExpiry)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateToken(user.ID.String(), org.ID.String(), string(user.Role), "refresh", s.refreshExpiry)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
		Organization: org,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshTokenStr string) (*domain.LoginResponse, error) {
	tokenClaims := &claims{}
	token, err := jwt.ParseWithClaims(refreshTokenStr, tokenClaims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, domain.ErrUnauthorized
	}

	if tokenClaims.RegisteredClaims.Subject != "refresh" {
		return nil, domain.ErrUnauthorized
	}

	if tokenClaims.UserID == "" || tokenClaims.OrgID == "" {
		return nil, domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(tokenClaims.UserID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	orgID, err := uuid.Parse(tokenClaims.OrgID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	user, err := s.userRepo.GetByID(ctx, userID, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, domain.ErrUnauthorized
	}

	org, err := s.orgRepo.GetByID(ctx, user.OrgID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	if !org.IsActive {
		return nil, domain.ErrUnauthorized
	}

	accessToken, err := s.generateToken(user.ID.String(), org.ID.String(), string(user.Role), "access", s.accessExpiry)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.generateToken(user.ID.String(), org.ID.String(), string(user.Role), "refresh", s.refreshExpiry)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User:         user,
		Organization: org,
	}, nil
}

func (s *authService) generateToken(userID, orgID, role, tokenType string, expiry time.Duration) (string, error) {
	tokenClaims := &claims{
		UserID: userID,
		OrgID:  orgID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   tokenType,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)
	return token.SignedString([]byte(s.jwtSecret))
}
