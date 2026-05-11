package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// Exercises hybrid PQC issue + validate path used by JWTMiddleware in production,
// without HTTP. Complements wallet_policy_context JWT handler tests (401 only).
func TestAuthService_generateToken_ValidateToken_roundTrip(t *testing.T) {
	t.Parallel()

	svc, err := NewAuthService(nil, nil, "unused-legacy-byte-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uid := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	email := "roundtrip-test@example.com"

	tok, err := svc.generateToken(uid, email)
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}

	claims, err := svc.ValidateToken(tok)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != uid {
		t.Fatalf("user_id mismatch: got %v want %v", claims.UserID, uid)
	}
	if claims.Email != email {
		t.Fatalf("email = %q, want %q", claims.Email, email)
	}
}
