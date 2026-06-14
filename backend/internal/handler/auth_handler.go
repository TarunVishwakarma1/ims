package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/utils"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AuthHandler struct {
	service  service.AuthService
	validate *validator.Validate
}

func NewAuthHandler(service service.AuthService) *AuthHandler {
	return &AuthHandler{
		service:  service,
		validate: validator.New(),
	}
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type VerifyEmailRequest struct {
	OTP string `json:"otp" validate:"required,len=6"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ip := utils.GetClientIP(r)
	ua := r.UserAgent()

	resp, err := h.service.Login(r.Context(), req.Email, req.Password, ip, ua)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		// Lockout error carries its own message
		if err.Error() == "account locked, try again later" {
			writeError(w, http.StatusLocked, err.Error())
			return
		}
		zap.L().Error("Login failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ip := utils.GetClientIP(r)
	ua := r.UserAgent()

	resp, err := h.service.RefreshToken(r.Context(), req.RefreshToken, ip, ua)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			writeError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		zap.L().Error("RefreshToken failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req service.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := utils.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ip := utils.GetClientIP(r)
	ua := r.UserAgent()

	resp, err := h.service.Signup(r.Context(), &req, ip, ua)
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			writeError(w, http.StatusConflict, "email or organization slug already exists")
			return
		}
		if errors.Is(err, utils.ErrPasswordPwned) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		zap.L().Error("Signup failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req RefreshTokenRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	_ = h.service.Logout(r.Context(), req.RefreshToken)
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	userIDStr, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid user")
		return
	}

	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.VerifyEmail(r.Context(), userID, req.OTP); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "email verified"})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid user")
		return
	}
	if _, err := h.service.ResendVerificationOTP(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not send OTP")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "OTP sent"})
}
