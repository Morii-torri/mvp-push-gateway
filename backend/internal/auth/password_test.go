package auth

import "testing"

func TestPasswordHashVerifiesOriginalPassword(t *testing.T) {
	hash, err := HashPassword("valid-password-123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	ok, err := VerifyPassword("valid-password-123", hash)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !ok {
		t.Fatal("expected original password to verify")
	}

	ok, err = VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to be rejected")
	}
}

func TestSessionTokenHashIsStable(t *testing.T) {
	token, err := NewSessionToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if HashSessionToken(token) != HashSessionToken(token) {
		t.Fatal("expected token hash to be stable")
	}
	if HashSessionToken(token) == token {
		t.Fatal("expected token hash to differ from raw token")
	}
}
