package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestMiddleware(t *testing.T) {
	id := uuid.New()
	valid, _ := GenerateToken(id, testSecret)
	foreign, _ := GenerateToken(id, "other-secret")

	cases := map[string]struct {
		header     string
		wantStatus int
	}{
		"no header":        {"", http.StatusUnauthorized},
		"not bearer":       {"Basic " + valid, http.StatusUnauthorized},
		"empty bearer":     {"Bearer ", http.StatusUnauthorized},
		"wrong secret":     {"Bearer " + foreign, http.StatusUnauthorized},
		"valid token":      {"Bearer " + valid, http.StatusOK},
		"lowercase bearer": {"bearer " + valid, http.StatusUnauthorized},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var gotID uuid.UUID
			var called bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotID, _ = UserIDFromContext(r.Context())
			})

			req := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			Middleware(testSecret)(next).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK {
				if !called || gotID != id {
					t.Fatalf("next called=%v with user %s, want %s", called, gotID, id)
				}
			} else if called {
				t.Fatal("next handler ran for an unauthenticated request")
			}
		})
	}
}

func TestUserIDFromContextMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := UserIDFromContext(req.Context()); ok {
		t.Fatal("expected no user ID in a bare context")
	}
}
