package access_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"rfp-agent/internal/access"
	"rfp-agent/internal/answers"
	"rfp-agent/internal/chat"
	"rfp-agent/internal/documents"
	"rfp-agent/internal/project"
	"rfp-agent/internal/user"
	"rfp-agent/internal/workspace"
)

var (
	alice = uuid.New()
	bob   = uuid.New()
)

// owners maps resource IDs to the user who owns them.
type owners map[uuid.UUID]uuid.UUID

func (o owners) Owns(_ context.Context, userID uuid.UUID, kind access.Kind, id uuid.UUID) (bool, error) {
	if kind == access.User {
		return id == userID, nil
	}
	return o[id] == userID, nil
}

type ctxKey struct{}

func userFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

// serve registers pattern on a guard and sends one request as the given user
// (uuid.Nil: no user). It returns the status and the body the handler saw.
func serve(t *testing.T, o owners, pattern, method, path, body string, as uuid.UUID) (int, string, bool) {
	t.Helper()
	mux := http.NewServeMux()
	called := false
	seen := ""
	access.NewGuard(mux, o, userFromCtx).HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		called = true
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
	})

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if as != uuid.Nil {
		req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, as))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec.Code, seen, called
}

func TestOwnerReachesTheirResource(t *testing.T) {
	proj := uuid.New()
	code, _, called := serve(t, owners{proj: alice}, "GET /projects/{id}", "GET", "/projects/"+proj.String(), "", alice)
	if code != http.StatusOK || !called {
		t.Fatalf("owner got %d (handler called: %v)", code, called)
	}
}

func TestOtherUserGetsNotFound(t *testing.T) {
	proj := uuid.New()
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		code, _, called := serve(t, owners{proj: alice}, method+" /projects/{id}", method, "/projects/"+proj.String(), "{}", bob)
		if code != http.StatusNotFound || called {
			t.Errorf("%s by another user: got %d (handler called: %v), want 404", method, code, called)
		}
	}
}

func TestMissingResourceIsNotFound(t *testing.T) {
	code, _, called := serve(t, owners{}, "GET /documents/{id}", "GET", "/documents/"+uuid.NewString(), "", alice)
	if code != http.StatusNotFound || called {
		t.Fatalf("got %d (handler called: %v), want 404", code, called)
	}
}

func TestNestedParamsAreChecked(t *testing.T) {
	ws := uuid.New()
	code, _, called := serve(t, owners{ws: alice}, "GET /workspaces/{workspaceID}/documents", "GET", "/workspaces/"+ws.String()+"/documents", "", bob)
	if code != http.StatusNotFound || called {
		t.Fatalf("got %d (handler called: %v), want 404", code, called)
	}
}

func TestBodyIDsAreChecked(t *testing.T) {
	mine, theirs := uuid.New(), uuid.New()
	o := owners{mine: alice, theirs: bob}

	body := `{"workspace_id":"` + mine.String() + `","project_id":"` + theirs.String() + `","question":"Q?"}`
	code, _, called := serve(t, o, "POST /answers", "POST", "/answers", body, alice)
	if code != http.StatusNotFound || called {
		t.Fatalf("someone else's project_id: got %d (handler called: %v), want 404", code, called)
	}

	body = `{"workspace_id":"` + mine.String() + `","question":"Q?"}`
	code, seen, called := serve(t, o, "POST /answers", "POST", "/answers", body, alice)
	if code != http.StatusOK || !called {
		t.Fatalf("own workspace_id: got %d (handler called: %v)", code, called)
	}
	if seen != body {
		t.Fatalf("handler saw body %q, want it unchanged", seen)
	}
}

func TestUnreadableBodyIsLeftToTheHandler(t *testing.T) {
	code, seen, called := serve(t, owners{}, "POST /projects", "POST", "/projects", "not json", alice)
	if code != http.StatusOK || !called || seen != "not json" {
		t.Fatalf("got %d, handler called %v with %q", code, called, seen)
	}
}

func TestOnlyYourOwnAccount(t *testing.T) {
	code, _, _ := serve(t, owners{}, "DELETE /users/{id}", "DELETE", "/users/"+alice.String(), "", bob)
	if code != http.StatusNotFound {
		t.Fatalf("deleting another account: got %d, want 404", code)
	}
	code, _, _ = serve(t, owners{}, "DELETE /users/{id}", "DELETE", "/users/"+alice.String(), "", alice)
	if code != http.StatusOK {
		t.Fatalf("deleting your own account: got %d", code)
	}
}

func TestNoUserIsUnauthorized(t *testing.T) {
	code, _, called := serve(t, owners{}, "POST /workspaces", "POST", "/workspaces", "{}", uuid.Nil)
	if code != http.StatusUnauthorized || called {
		t.Fatalf("got %d (handler called: %v), want 401", code, called)
	}
}

func TestUnknownParamPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a route with an unmapped {param} registered without an owner rule")
		}
	}()
	access.NewGuard(http.NewServeMux(), owners{}, userFromCtx).HandleFunc("GET /widgets/{id}", func(http.ResponseWriter, *http.Request) {})
}

// Every real route must map to an owner rule, or the server would panic at startup.
func TestAllRoutesHaveOwnerRules(t *testing.T) {
	g := access.NewGuard(http.NewServeMux(), owners{}, userFromCtx)
	user.NewHandler(nil).RegisterRoutes(g)
	workspace.NewHandler(nil).RegisterRoutes(g)
	project.NewHandler(nil).RegisterRoutes(g)
	documents.NewHandler(nil).RegisterRoutes(g)
	answers.NewHandler(nil).RegisterRoutes(g)
	chat.NewHandler(nil).RegisterRoutes(g)
}
