package httpapi

import (
	"context"
	"testing"
	"time"
)

func TestAuthManagerGenerateAndParseToken(t *testing.T) {
	auth := NewAuthManager("test-secret", time.Hour)

	token, err := auth.GenerateToken("user-1", "User@Example.COM")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := auth.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}

	if claims.UserID != "user-1" {
		t.Fatalf("claims.UserID = %q, want %q", claims.UserID, "user-1")
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("claims.Email = %q, want %q", claims.Email, "user@example.com")
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		t.Fatalf("claims must contain exp and iat")
	}
	if !claims.ExpiresAt.After(claims.IssuedAt.Time) {
		t.Fatalf("expires_at must be after issued_at")
	}
}

func TestAuthManagerParseTokenWithWrongSecretFails(t *testing.T) {
	authA := NewAuthManager("secret-a", time.Hour)
	authB := NewAuthManager("secret-b", time.Hour)

	token, err := authA.GenerateToken("user-1", "user@example.com")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if _, err := authB.ParseToken(token); err == nil {
		t.Fatalf("ParseToken() expected error for wrong secret, got nil")
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("strong-pass-123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "strong-pass-123" {
		t.Fatalf("hash must not equal raw password")
	}

	if err := VerifyPassword(hash, "strong-pass-123"); err != nil {
		t.Fatalf("VerifyPassword() valid password error = %v", err)
	}

	if err := VerifyPassword(hash, "wrong-pass"); err == nil {
		t.Fatalf("VerifyPassword() expected error for invalid password, got nil")
	}
}

func TestCurrentUserContextRoundTrip(t *testing.T) {
	ctx := withCurrentUser(context.Background(), CurrentUser{
		ID:    "user-1",
		Email: "user@example.com",
	})

	got, ok := currentUserFromContext(ctx)
	if !ok {
		t.Fatalf("currentUserFromContext() ok = false, want true")
	}
	if got.ID != "user-1" || got.Email != "user@example.com" {
		t.Fatalf("currentUserFromContext() = %#v, want ID=user-1 Email=user@example.com", got)
	}
}

func TestCurrentUserContextMissing(t *testing.T) {
	_, ok := currentUserFromContext(context.Background())
	if ok {
		t.Fatalf("currentUserFromContext() ok = true, want false")
	}
}
