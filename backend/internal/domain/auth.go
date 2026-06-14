package domain

type LoginResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	User         *User         `json:"user"`
	Organization *Organization `json:"organization"`
}
