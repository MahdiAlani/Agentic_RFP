package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "test-secret"

func TestGenerateAndParseToken(t *testing.T) {
	id := uuid.New()
	tok, err := GenerateToken(id, testSecret)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	got, err := ParseToken(tok, testSecret)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if got != id {
		t.Fatalf("subject = %s, want %s", got, id)
	}
}

func TestParseTokenRejects(t *testing.T) {
	id := uuid.New()
	valid, _ := GenerateToken(id, testSecret)

	expired, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   id.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}).SignedString([]byte(testSecret))

	// "alg: none" must never be accepted, whatever the claims say.
	unsigned, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject: id.String(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	badSubject, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "not-a-uuid",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte(testSecret))

	cases := map[string]struct {
		token, secret string
	}{
		"wrong secret":     {valid, "other-secret"},
		"expired":          {expired, testSecret},
		"alg none":         {unsigned, testSecret},
		"non-uuid subject": {badSubject, testSecret},
		"garbage":          {"not.a.jwt", testSecret},
		"empty":            {"", testSecret},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseToken(tc.token, tc.secret); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}
