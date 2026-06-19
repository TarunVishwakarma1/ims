package domain

// LoginResponse is returned by both the initial /login and the second-step
// /login/verify-2fa. RequireTOTP=true → no access/refresh tokens yet,
// caller must POST {pending_token, code} to /api/auth/login/verify-2fa.
type LoginResponse struct {
	AccessToken  string        `json:"access_token,omitempty"`
	RefreshToken string        `json:"refresh_token,omitempty"`
	User         *User         `json:"user,omitempty"`
	Organization *Organization `json:"organization,omitempty"`

	RequireTOTP  bool   `json:"require_totp,omitempty"`
	PendingToken string `json:"pending_token,omitempty"`
	// TwoFAMethod hints to the client which UI to show next: "totp" or
	// "email". For email, an OTP has already been sent to the address on
	// the account.
	TwoFAMethod string `json:"two_fa_method,omitempty"`
}
