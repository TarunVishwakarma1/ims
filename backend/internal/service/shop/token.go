package shop

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const ShopAudience = "shop"

type shopClaims struct {
	jwt.RegisteredClaims
}

func IssueShopJWT(secret string, customerID uuid.UUID, ttl time.Duration) (string, error) {
	c := shopClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "ims",
			Subject:   customerID.String(),
			Audience:  jwt.ClaimStrings{ShopAudience},
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString([]byte(secret))
}

func ParseShopJWT(secret, token string) (uuid.UUID, error) {
	c := &shopClaims{}
	tok, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return uuid.Nil, err
	}
	if !hasAudience(c.Audience, ShopAudience) {
		return uuid.Nil, errors.New("wrong audience")
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func hasAudience(auds jwt.ClaimStrings, want string) bool {
	for _, a := range auds {
		if a == want {
			return true
		}
	}
	return false
}
