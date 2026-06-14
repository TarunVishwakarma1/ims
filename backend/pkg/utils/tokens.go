package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
)

// GenerateRandomToken returns a cryptographically secure URL-safe token of
// `byteLen` random bytes. 32 bytes ≈ 256 bits of entropy — refresh-token grade.
func GenerateRandomToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns hex-encoded sha256(token). We never store raw refresh tokens.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ConstantTimeCompare compares two strings without leaking timing info.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// GenerateOTP returns a 6-digit numeric OTP using crypto/rand.
// math/rand is only used as fallback if crypto fails (vanishingly rare).
func GenerateOTP() string {
	const digits = "0123456789"
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		// fall back to math/rand (should never happen)
		for i := range buf {
			buf[i] = digits[mathrand.Intn(len(digits))]
		}
	} else {
		for i := range buf {
			buf[i] = digits[int(buf[i])%len(digits)]
		}
	}
	return fmt.Sprintf("%s", string(buf))
}
