package shop

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	srv "github.com/TarunVishwakarma1/ims/backend/internal/service/shop"
)

type AuthHandler struct {
	otp srv.OTPService
}

func NewAuthHandler(o srv.OTPService) *AuthHandler {
	return &AuthHandler{otp: o}
}

type sendReq struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

func (h *AuthHandler) Send(w http.ResponseWriter, r *http.Request) {
	var req sendReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}

	purpose := req.Purpose
	if purpose == "" {
		purpose = "login"
	}

	id, exp, err := h.otp.Send(r.Context(), req.Phone, purpose)
	if err != nil {
		switch {
		case errors.Is(err, srv.ErrInvalidPhone):
			writeErr(w, http.StatusBadRequest, "invalid_phone")
		case errors.Is(err, srv.ErrRateLimited):
			writeErr(w, http.StatusTooManyRequests, "rate_limit")
		default:
			writeErr(w, http.StatusInternalServerError, "send_failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"otp_id": id, "expires_in": exp})
}

type verifyReq struct {
	OTPID string `json:"otp_id"`
	Code  string `json:"code"`
}

func (h *AuthHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req verifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body")
		return
	}

	id, err := uuid.Parse(req.OTPID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_otp_id")
		return
	}

	cid, tok, err := h.otp.Verify(r.Context(), id, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, srv.ErrInvalidCode):
			writeErr(w, http.StatusBadRequest, "invalid_code")
		case errors.Is(err, srv.ErrOTPExpired):
			writeErr(w, http.StatusGone, "otp_expired")
		case errors.Is(err, srv.ErrOTPLocked):
			writeErr(w, http.StatusLocked, "otp_locked")
		default:
			writeErr(w, http.StatusInternalServerError, "verify_failed")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token": tok,
		"customer": map[string]any{
			"id": cid,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
