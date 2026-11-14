// Package access makes sure a user only reaches their own data.
//
// Everything a user owns hangs off a workspace (workspaces.user_id), so a
// project, document, answer or conversation belongs to whoever owns its
// workspace. Guard wraps every protected route and checks each ID the request
// names, in the path or in a JSON body, before the handler runs. Handlers stay
// unaware of it.
package access

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Kind string

const (
	Workspace    Kind = "workspace"
	Project      Kind = "project"
	Document     Kind = "document"
	Answer       Kind = "answer"
	Conversation Kind = "conversation"
	User         Kind = "user"
)

// Router is what handlers register routes on: a Guard, or a plain
// *http.ServeMux in tests.
type Router interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Checker answers whether userID owns the resource. A resource that doesn't
// exist is not owned.
type Checker interface {
	Owns(ctx context.Context, userID uuid.UUID, kind Kind, id uuid.UUID) (bool, error)
}

// Guard registers routes on mux, each wrapped in an ownership check.
type Guard struct {
	mux    *http.ServeMux
	check  Checker
	userID func(context.Context) (uuid.UUID, bool)
}

// NewGuard takes the caller's ID from the request context with userID
// (auth.UserIDFromContext; passed in because auth already imports handlers
// that import this package).
func NewGuard(mux *http.ServeMux, check Checker, userID func(context.Context) (uuid.UUID, bool)) *Guard {
	return &Guard{mux: mux, check: check, userID: userID}
}

// maxBody caps how much of a request body Guard reads to find IDs. API bodies
// are small JSON; file bytes go straight to object storage.
const maxBody = 1 << 20

// bodyFields are the JSON body fields that name a resource the caller must own.
var bodyFields = map[string]Kind{
	"workspace_id": Workspace,
	"project_id":   Project,
	"document_id":  Document,
}

type target struct {
	param string
	kind  Kind
}

// HandleFunc panics if the pattern has a path parameter Guard can't map to a
// resource, so a new route can't go unchecked by accident.
func (g *Guard) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	targets, err := pathTargets(pattern)
	if err != nil {
		panic(err)
	}

	g.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		userID, ok := g.userID(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		for _, t := range targets {
			id, err := uuid.Parse(r.PathValue(t.param))
			if err != nil {
				continue // not a UUID: the handler answers 400
			}
			if !g.allowed(w, r, userID, t.kind, id) {
				return
			}
		}

		if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodDelete {
			raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
			if err != nil {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			if len(raw) > maxBody {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))

			for kind, id := range bodyIDs(raw) {
				if !g.allowed(w, r, userID, kind, id) {
					return
				}
			}
		}

		handler(w, r)
	})
}

// allowed writes the error response and returns false when userID may not
// touch the resource. Someone else's resource is reported as not found, so
// its existence doesn't leak.
func (g *Guard) allowed(w http.ResponseWriter, r *http.Request, userID uuid.UUID, kind Kind, id uuid.UUID) bool {
	owns, err := g.check.Owns(r.Context(), userID, kind, id)
	if err != nil {
		log.Printf("%s %s: ownership check: %v", r.Method, r.URL.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return false
	}
	if !owns {
		http.Error(w, "not found", http.StatusNotFound)
		return false
	}
	return true
}

// pathTargets maps each {param} in a pattern like "GET /projects/{id}/answers"
// to the kind of resource it names.
func pathTargets(pattern string) ([]target, error) {
	_, path, _ := strings.Cut(pattern, " ")
	segments := strings.Split(strings.Trim(path, "/"), "/")

	var targets []target
	for i, seg := range segments {
		param, ok := strings.CutPrefix(seg, "{")
		if !ok {
			continue
		}
		param = strings.TrimSuffix(param, "}")

		var kind Kind
		switch param {
		case "workspaceID":
			kind = Workspace
		case "projectID":
			kind = Project
		case "id":
			if i > 0 {
				kind = collections[segments[i-1]]
			}
		}
		if kind == "" {
			return nil, fmt.Errorf("access: no owner rule for {%s} in %q", param, pattern)
		}
		targets = append(targets, target{param: param, kind: kind})
	}
	return targets, nil
}

var collections = map[string]Kind{
	"workspaces":    Workspace,
	"projects":      Project,
	"documents":     Document,
	"answers":       Answer,
	"conversations": Conversation,
	"users":         User,
}

// bodyIDs returns the resource IDs named in a JSON object body. Anything that
// isn't a JSON object, or a field that isn't a UUID, is left for the handler
// to reject.
func bodyIDs(raw []byte) map[Kind]uuid.UUID {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return nil
	}
	ids := map[Kind]uuid.UUID{}
	for field, kind := range bodyFields {
		s, ok := body[field].(string)
		if !ok {
			continue
		}
		if id, err := uuid.Parse(s); err == nil {
			ids[kind] = id
		}
	}
	return ids
}

// DB checks ownership in Postgres.
type DB struct {
	pool *pgxpool.Pool
}

func NewDB(pool *pgxpool.Pool) *DB {
	return &DB{pool: pool}
}

// ownerQueries find the user who owns a resource, through its workspace.
var ownerQueries = map[Kind]string{
	Workspace: `SELECT user_id FROM workspaces WHERE id = $1`,
	Project: `SELECT w.user_id FROM projects p
		JOIN workspaces w ON w.id = p.workspace_id WHERE p.id = $1`,
	Document: `SELECT w.user_id FROM documents d
		JOIN workspaces w ON w.id = d.workspace_id WHERE d.id = $1`,
	Answer: `SELECT w.user_id FROM answers a
		JOIN workspaces w ON w.id = a.workspace_id WHERE a.id = $1`,
	Conversation: `SELECT w.user_id FROM conversations c
		JOIN projects p ON p.id = c.project_id
		JOIN workspaces w ON w.id = p.workspace_id WHERE c.id = $1`,
}

func (d *DB) Owns(ctx context.Context, userID uuid.UUID, kind Kind, id uuid.UUID) (bool, error) {
	if kind == User {
		return id == userID, nil
	}
	query, ok := ownerQueries[kind]
	if !ok {
		return false, fmt.Errorf("access: unknown kind %q", kind)
	}
	var owner uuid.UUID
	err := d.pool.QueryRow(ctx, query, id).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return owner == userID, nil
}
