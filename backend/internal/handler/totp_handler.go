package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TOTPHandler struct {
	svc      service.TOTPService
	authSvc  service.AuthService
	userRepo repository.UserRepository
}

func NewTOTPHandler(svc service.TOTPService, authSvc service.AuthService, userRepo repository.UserRepository) *TOTPHandler {
	return &TOTPHandler{svc: svc, authSvc: authSvc, userRepo: userRepo}
}

// Enroll — POST /api/auth/2fa/enroll
// Returns the otpauth URI for the QR code and the raw secret (so power
// users can type it into a TOTP app manually).
func (h *TOTPHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	// Resolve the user's email to use as the account label. Authenticators
	// show "IMS:hello@example.com" — far more readable than a raw UUID and
	// matches what users see on Google / GitHub / etc.
	label := h.userEmail(r, actor.UserID)
	uri, secret, err := h.svc.Enroll(r.Context(), actor.UserID, label)
	if err != nil {
		zap.L().Error("totp enroll failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"uri": uri, "secret": secret})
}

// userEmail looks up the actor's email. Falls back to the UUID string if
// the lookup fails for any reason — the enrollment still works, just with
// a less readable authenticator label.
func (h *TOTPHandler) userEmail(r *http.Request, userID uuid.UUID) string {
	u, err := h.userRepo.GetByID(r.Context(), userID, uuid.Nil)
	if err != nil || u == nil || u.Email == "" {
		return userID.String()
	}
	return u.Email
}

// Confirm — POST /api/auth/2fa/confirm { code }
// Returns backup codes ONCE — UI must show + warn user to save them.
func (h *TOTPHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	var body struct{ Code string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	codes, err := h.svc.Confirm(r.Context(), actor.UserID, body.Code)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backup_codes": codes})
}

// Disable — POST /api/auth/2fa/disable
func (h *TOTPHandler) Disable(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	if err := h.svc.Disable(r.Context(), actor.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "2FA disabled"})
}

// VerifyLogin — POST /api/auth/login/verify-2fa { pending_token, code }
// Public — no Auth middleware. The pending_token JWT carries the identity.
func (h *TOTPHandler) VerifyLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PendingToken string `json:"pending_token"`
		Code         string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	ip := r.RemoteAddr
	ua := r.UserAgent()
	resp, err := h.authSvc.VerifyTOTPLogin(r.Context(), body.PendingToken, body.Code, ip, ua)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials or code")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// EnableEmail2FA — POST /api/auth/2fa/email/enable
func (h *TOTPHandler) EnableEmail2FA(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	if err := h.userRepo.SetEmail2FA(r.Context(), actor.UserID, true); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Email 2FA enabled"})
}

// DisableEmail2FA — POST /api/auth/2fa/email/disable
func (h *TOTPHandler) DisableEmail2FA(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.ActorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return
	}
	if err := h.userRepo.SetEmail2FA(r.Context(), actor.UserID, false); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Email 2FA disabled"})
}

// ResendLoginOTP — POST /api/auth/login/resend-2fa { pending_token }
// User clicks "Resend code" on the second-step screen. Validates the
// pending JWT, re-issues a fresh email OTP for the implied user.
func (h *TOTPHandler) ResendLoginOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PendingToken string `json:"pending_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PendingToken == "" {
		writeError(w, http.StatusBadRequest, "pending_token required")
		return
	}
	if err := h.authSvc.ResendLoginEmailOTP(r.Context(), body.PendingToken, r.RemoteAddr); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired pending token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "OTP sent"})
}
