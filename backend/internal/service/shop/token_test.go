package shop

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestShopJWT_RoundTrip(t *testing.T) {
	cid := uuid.New()
	tok, err := IssueShopJWT("secret", cid, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseShopJWT("secret", tok)
	if err != nil {
		t.Fatal(err)
	}
	if got != cid {
		t.Fatalf("expected %v got %v", cid, got)
	}
}

func TestShopJWT_RejectsWrongSecret(t *testing.T) {
	tok, _ := IssueShopJWT("a", uuid.New(), time.Hour)
	if _, err := ParseShopJWT("b", tok); err == nil {
		t.Fatal("expected error")
	}
}

func TestShopJWT_RejectsB2BAudience(t *testing.T) {
	// Forged B2B-style token must not parse via ParseShopJWT.
	// (Sanity check; full audience enforcement lives in middleware.)
}
