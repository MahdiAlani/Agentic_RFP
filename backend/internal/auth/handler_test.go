package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"rfp-agent/internal/user"
)

// fakeUsers stubs the methods the auth handler calls; anything else panics
// through the nil embedded interface.
type fakeUsers struct {
	user.Service
	createErr error
	authErr   error
	u         *user.User
}

func (f *fakeUsers) CreateUser(_ context.Context, email, name, _ string) (*user.User, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &user.User{ID: f.u.ID, Email: email, Name: name}, nil
}

func (f *fakeUsers) Authenticate(context.Context, string, string) (*user.User, error) {
	if f.authErr != nil {
		return nil, f.authErr
	}
	return f.u, nil
}

func (f *fakeUsers) GetUser(_ context.Context, id uuid.UUID) (*user.User, error) {
	if id != f.u.ID {
		return nil, user.ErrNotFound
	}
	return f.u, nil
}

func newTestMux(users user.Service) *http.ServeMux {
	h := NewHandler(users, testSecret)
	protected := http.NewServeMux()
	h.RegisterProtectedRoutes(protected)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	mux.Handle("/", Middleware(testSecret)(protected))
	return mux
}

func do(t *testing.T, mux http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRegisterAndLoginIssueUsableToken(t *testing.T) {
	u := &user.User{ID: uuid.New(), Email: "a@b.co", Name: "A"}
	mux := newTestMux(&fakeUsers{u: u})

	for _, tc := range []struct {
		path string
		want int
	}{
		{"/auth/register", http.StatusCreated},
		{"/auth/login", http.StatusOK},
	} {
		rec := do(t, mux, http.MethodPost, tc.path, `{"email":"a@b.co","name":"A","password":"password1"}`, "")
		if rec.Code != tc.want {
			t.Fatalf("%s status = %d, want %d (%s)", tc.path, rec.Code, tc.want, rec.Body)
		}

		var resp struct {
			Token string          `json:"token"`
			User  json.RawMessage `json:"user"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: decode: %v", tc.path, err)
		}
		if strings.Contains(string(resp.User), "password") {
			t.Fatalf("%s leaked a password field: %s", tc.path, resp.User)
		}

		got, err := ParseToken(resp.Token, testSecret)
		if err != nil || got != u.ID {
			t.Fatalf("%s token subject = %s, err %v; want %s", tc.path, got, err, u.ID)
		}

		me := do(t, mux, http.MethodGet, "/auth/me", "", resp.Token)
		if me.Code != http.StatusOK {
			t.Fatalf("/auth/me with %s token: status %d", tc.path, me.Code)
		}
	}
}

func TestAuthErrorStatuses(t *testing.T) {
	u := &user.User{ID: uuid.New()}
	cases := map[string]struct {
		users *fakeUsers
		path  string
		body  string
		want  int
	}{
		"register bad json":  {&fakeUsers{u: u}, "/auth/register", `{`, http.StatusBadRequest},
		"register invalid":   {&fakeUsers{u: u, createErr: user.ErrInvalidInput}, "/auth/register", `{}`, http.StatusBadRequest},
		"register duplicate": {&fakeUsers{u: u, createErr: user.ErrConflict}, "/auth/register", `{}`, http.StatusConflict},
		"login bad password": {&fakeUsers{u: u, authErr: user.ErrInvalidCredentials}, "/auth/login", `{}`, http.StatusUnauthorized},
		"login db failure":   {&fakeUsers{u: u, authErr: context.DeadlineExceeded}, "/auth/login", `{}`, http.StatusInternalServerError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := do(t, newTestMux(tc.users), http.MethodPost, tc.path, tc.body, "")
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusInternalServerError && strings.Contains(rec.Body.String(), "deadline") {
				t.Fatalf("internal error detail leaked to client: %q", rec.Body)
			}
		})
	}
}

func TestMeRequiresToken(t *testing.T) {
	rec := do(t, newTestMux(&fakeUsers{u: &user.User{ID: uuid.New()}}), http.MethodGet, "/auth/me", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
