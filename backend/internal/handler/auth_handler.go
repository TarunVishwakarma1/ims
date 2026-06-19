package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/TarunVishwakarma1/ims/backend/pkg/utils"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AuthHandler struct {
	service  service.AuthService
	userRepo repository.UserRepository
	validate *validator.Validate
}

func NewAuthHandler(service service.AuthService, userRepo repository.UserRepository) *AuthHandler {
	return &AuthHandler{
		service:  service,
		userRepo: userRepo,
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

// logger returns a zap logger annotated with request_id + path so every line
// is correlatable in production logs.
func logger(r *http.Request) *zap.Logger {
	return zap.L().With(
		zap.String("request_id", chiMiddleware.GetReqID(r.Context())),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.String("ip", utils.GetClientIP(r)),
	)
}

// formatValidationErr turns the verbose validator output into a single
// human-readable line per failed field.
func formatValidationErr(err error) string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err.Error()
	}
	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		field := strings.ToLower(fe.Field())
		switch fe.Tag() {
		case "required":
			msgs = append(msgs, field+" is required")
		case "email":
			msgs = append(msgs, field+" must be a valid email")
		case "min":
			msgs = append(msgs, field+" must be at least "+fe.Param()+" chars")
		case "max":
			msgs = append(msgs, field+" must be at most "+fe.Param()+" chars")
		case "len":
			msgs = append(msgs, field+" must be exactly "+fe.Param()+" chars")
		default:
			msgs = append(msgs, field+" is invalid ("+fe.Tag()+")")
		}
	}
	return strings.Join(msgs, "; ")
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	log := logger(r)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("login: invalid body", zap.Error(err))
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		msg := formatValidationErr(err)
		log.Warn("login: validation failed", zap.String("reason", msg))
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	log = log.With(zap.String("email", req.Email))
	ip := utils.GetClientIP(r)
	ua := r.UserAgent()

	resp, err := h.service.Login(r.Context(), req.Email, req.Password, ip, ua)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUnauthorized):
			log.Info("login: invalid credentials")
			writeError(w, http.StatusUnauthorized, "invalid email or password")
		case err.Error() == "account locked, try again later":
			log.Warn("login: account locked")
			writeError(w, http.StatusLocked, err.Error())
		default:
			log.Error("login: server error", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error: "+err.Error())
		}
		return
	}
	if resp.RequireTOTP {
		log.Info("login: 2fa required", zap.String("method", resp.TwoFAMethod))
	} else {
		log.Info("login: success", zap.String("user_id", resp.User.ID.String()))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	log := logger(r)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("refresh: invalid body", zap.Error(err))
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		msg := formatValidationErr(err)
		log.Warn("refresh: validation failed", zap.String("reason", msg))
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	ip := utils.GetClientIP(r)
	ua := r.UserAgent()

	resp, err := h.service.RefreshToken(r.Context(), req.RefreshToken, ip, ua)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			log.Info("refresh: invalid token")
			writeError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		log.Error("refresh: server error", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error: "+err.Error())
		return
	}
	log.Info("refresh: success")
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	log := logger(r)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req service.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("signup: invalid body", zap.Error(err))
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		msg := formatValidationErr(err)
		log.Warn("signup: validation failed", zap.String("reason", msg))
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := utils.ValidatePassword(req.Password); err != nil {
		log.Info("signup: weak password", zap.String("reason", err.Error()))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log = log.With(zap.String("email", req.Email), zap.String("slug", req.OrgSlug))
	ip := utils.GetClientIP(r)
	ua := r.UserAgent()

	resp, err := h.service.Signup(r.Context(), &req, ip, ua)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrConflict):
			log.Info("signup: conflict (email or slug taken)")
			writeError(w, http.StatusConflict, "email or organization slug already exists")
		case errors.Is(err, utils.ErrPasswordPwned):
			log.Info("signup: pwned password")
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			log.Error("signup: server error", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal server error: "+err.Error())
		}
		return
	}
	log.Info("signup: success",
		zap.String("user_id", resp.User.ID.String()),
		zap.String("org_id", resp.Organization.ID.String()))
	writeJSON(w, http.StatusCreated, resp)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req RefreshTokenRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	_ = h.service.Logout(r.Context(), req.RefreshToken)
	logger(r).Info("logout")
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	log := logger(r)
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
		writeError(w, http.StatusBadRequest, formatValidationErr(err))
		return
	}

	if err := h.service.VerifyEmail(r.Context(), userID, req.OTP); err != nil {
		log.Warn("verify-email failed", zap.Error(err))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Info("verify-email success", zap.String("user_id", userID.String()))
	writeJSON(w, http.StatusOK, map[string]string{"message": "email verified"})
}

func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	log := logger(r)
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
		log.Error("resend OTP failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "could not send OTP")
		return
	}
	log.Info("resend OTP success", zap.String("user_id", userID.String()))
	writeJSON(w, http.StatusOK, map[string]string{"message": "OTP sent"})
}

// RequestPasswordReset — POST /api/auth/password-reset/request {email}
// Always returns 200 + "if your email is registered, a link has been sent"
// to defend against account enumeration.
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	ip := utils.GetClientIP(r)
	_ = h.service.RequestPasswordReset(r.Context(), body.Email, ip, r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "If that email is registered, a reset link has been sent.",
	})
}

// ConfirmPasswordReset — POST /api/auth/password-reset/confirm {token, password}
func (h *AuthHandler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(body.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if err := h.service.ConfirmPasswordReset(r.Context(), body.Token, body.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "reset link is invalid or has expired")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Password updated. You can sign in now."})
}

// Me — GET /api/auth/me
// Returns the authenticated user. Used by the web app to rehydrate
// auth-store after page reload so server-side toggles (totp_enabled,
// email_verified, role) reflect on the client.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	u, err := h.userRepo.GetByID(r.Context(), actor.UserID, actor.OrgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, u)
}
