package utils

import (
	"bufio"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrPasswordPwned indicates the password appears in known breach corpora.
var ErrPasswordPwned = errors.New("password has appeared in a known data breach; please choose a different one")

// CheckPwnedPassword queries the HaveIBeenPwned k-anonymity API.
// Only the first 5 chars of the SHA-1 are sent; the API returns a list of
// suffixes. We compare client-side. The full password and full hash never leave the box.
//
// Returns ErrPasswordPwned if the password is breached, nil if safe, or an
// error for network/parse failures (which callers may choose to fail-open).
func CheckPwnedPassword(ctx context.Context, password string) error {
	hashBytes := sha1.Sum([]byte(password))
	hashHex := strings.ToUpper(fmt.Sprintf("%x", hashBytes))
	prefix := hashHex[:5]
	suffix := hashHex[5:]

	httpCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodGet,
		"https://api.pwnedpasswords.com/range/"+prefix, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Add-Padding", "true") // privacy padding from HIBP
	req.Header.Set("User-Agent", "ims-backend")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pwned check returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		// Format: "SUFFIX:COUNT"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(parts[0], suffix) {
			return ErrPasswordPwned
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}
	return nil
}
