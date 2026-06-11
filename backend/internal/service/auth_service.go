package service

import (
	"context"
	"errors"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Login(ctx context.Context, email, password, ipAddress string) (*domain.LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*domain.LoginResponse, error)
}

type authService struct {
	userRepo      repository.UserRepository
	jwtSecret     string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewAuthService(
	userRepo repository.UserRepository,
	jwtSecret string,
	accessExpiry time.Duration,
	refreshExpiry time.Duration,
) AuthService {
	return &authService{
		userRepo:      userRepo,
		jwtSecret:     jwtSecret,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

type claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
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

	accessToken, err := s.generateToken(user.ID.String(), string(user.Role), "access", s.accessExpiry)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateToken(user.ID.String(), string(user.Role), "refresh", s.refreshExpiry)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
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

	if tokenClaims.UserID == "" {
		return nil, domain.ErrUnauthorized
	}

	userID, err := uuid.Parse(tokenClaims.UserID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, domain.ErrUnauthorized
	}

	accessToken, err := s.generateToken(user.ID.String(), string(user.Role), "access", s.accessExpiry)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.generateToken(user.ID.String(), string(user.Role), "refresh", s.refreshExpiry)
	if err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User:         user,
	}, nil
}

func (s *authService) generateToken(userID, role, tokenType string, expiry time.Duration) (string, error) {
	tokenClaims := &claims{
		UserID: userID,
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
