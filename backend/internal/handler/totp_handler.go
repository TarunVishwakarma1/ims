package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TarunVishwakarma1/ims/backend/internal/service"
	"github.com/TarunVishwakarma1/ims/backend/pkg/middleware"
	"go.uber.org/zap"
)

type TOTPHandler struct {
	svc     service.TOTPService
	authSvc service.AuthService
}

func NewTOTPHandler(svc service.TOTPService, authSvc service.AuthService) *TOTPHandler {
	return &TOTPHandler{svc: svc, authSvc: authSvc}
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
	// Use the user id as the account name — keeps the entry uniquely
	// labeled in authenticators when a user has multiple IMS accounts.
	uri, secret, err := h.svc.Enroll(r.Context(), actor.UserID, actor.UserID.String())
	if err != nil {
		zap.L().Error("totp enroll failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"uri": uri, "secret": secret})
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
