package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yashs/mobile-gmail-notification/internal/auth"
)

func TestIssueAndParseAccessToken(t *testing.T) {
	tm := auth.NewTokenManager("super-secret-key-with-32-bytes!!", 15*time.Minute, 24*time.Hour)
	uid := uuid.New()
	token, exp, err := tm.IssueAccessToken(uid, "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if exp.Before(time.Now()) {
		t.Fatal("expiry in the past")
	}

	claims, err := tm.ParseAccessToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != uid {
		t.Fatalf("uid mismatch")
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("email mismatch")
	}
}

func TestParseRejectsTamperedToken(t *testing.T) {
	tm := auth.NewTokenManager("super-secret-key-with-32-bytes!!", time.Minute, time.Hour)
	token, _, err := tm.IssueAccessToken(uuid.New(), "a@b.com")
	if err != nil {
		t.Fatal(err)
	}
	tampered := token + "x"
	if _, err := tm.ParseAccessToken(tampered); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestTokenStillValid(t *testing.T) {
	if !auth.TokenStillValid(time.Now().Add(time.Hour)) {
		t.Fatal("expected valid")
	}
	if auth.TokenStillValid(time.Now().Add(-time.Minute)) {
		t.Fatal("expected expired")
	}
}
