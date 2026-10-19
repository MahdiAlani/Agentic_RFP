package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// route matches registrations like mux.HandleFunc("GET /answers/{id}", ...).
var route = regexp.MustCompile(`HandleFunc\("([A-Z]+) (/[^"]*)"`)

// registeredRoutes scans every handler for the routes it wires up.
func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("../internal/*/handler.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no handlers found: %v", err)
	}
	routes := map[string]bool{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range route.FindAllStringSubmatch(string(src), -1) {
			routes[m[1]+" "+m[2]] = true
		}
	}
	return routes
}

func documentedRoutes(t *testing.T) map[string]bool {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatalf("openapi.yaml does not parse: %v", err)
	}
	routes := map[string]bool{}
	for path, ops := range doc.Paths {
		for method := range ops {
			if method != "parameters" {
				routes[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	return routes
}

func missingFrom(want, have map[string]bool) []string {
	var out []string
	for r := range want {
		if !have[r] {
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

// The spec must describe exactly the routes the API serves, so it can't drift.
func TestSpecMatchesRegisteredRoutes(t *testing.T) {
	registered, documented := registeredRoutes(t), documentedRoutes(t)
	if missing := missingFrom(registered, documented); len(missing) > 0 {
		t.Errorf("routes missing from openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if stale := missingFrom(documented, registered); len(stale) > 0 {
		t.Errorf("openapi.yaml documents routes that don't exist:\n  %s", strings.Join(stale, "\n  "))
	}
}

func TestServesSpecAndDocs(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux)

	for path, wantType := range map[string]string{"/openapi.yaml": "application/yaml", "/docs": "text/html"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), wantType) {
			t.Errorf("%s: %d %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
}
