package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"rfp-agent/internal/user"
)

type Handler struct {
	users  user.Service
	secret string
}

func NewHandler(users user.Service, secret string) *Handler {
	return &Handler{users: users, secret: secret}
}

// RegisterRoutes wires the public auth endpoints (no token required).
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("POST /auth/login", h.Login)
}

// RegisterProtectedRoutes wires endpoints that require a valid token.
func (h *Handler) RegisterProtectedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/me", h.Me)
}

type authResponse struct {
	Token string     `json:"token"`
	User  *user.User `json:"user"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	u, err := h.users.CreateUser(r.Context(), body.Email, body.Name, body.Password)
	if err != nil {
		writeUserError(w, r, err)
		return
	}

	h.issueToken(w, r, u, http.StatusCreated)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	u, err := h.users.Authenticate(r.Context(), body.Email, body.Password)
	if err != nil {
		writeUserError(w, r, err)
		return
	}

	h.issueToken(w, r, u, http.StatusOK)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.users.GetUser(r.Context(), userID)
	if err != nil {
		writeUserError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, u)
}

func (h *Handler) issueToken(w http.ResponseWriter, r *http.Request, u *user.User, status int) {
	token, err := GenerateToken(u.ID, h.secret)
	if err != nil {
		log.Printf("%s %s: generate token: %v", r.Method, r.URL.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, status, authResponse{Token: token, User: u})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeUserError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, user.ErrInvalidCredentials):
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
	case errors.Is(err, user.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, user.ErrInvalidInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, user.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
