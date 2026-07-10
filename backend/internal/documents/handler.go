package document

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// mux to handle the endpoints
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /documents/{id}", h.Getdocument)
	mux.HandleFunc("GET /workspaces/{workspaceID}/documents", h.Listdocuments)
	mux.HandleFunc("POST /documents", h.Createdocument)
	mux.HandleFunc("PUT /documents/{id}", h.Updatedocument)
	mux.HandleFunc("DELETE /documents/{id}", h.Deletedocument)
}

func (h *Handler) Getdocument(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	// Was not a UUID
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	ws, err := h.svc.Getdocument(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

func (h *Handler) Listdocuments(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := uuid.Parse(r.PathValue("workspaceID"))
	if err != nil {
		http.Error(w, "invalid workspace id", http.StatusBadRequest)
		return
	}

	ws, err := h.svc.Listdocuments(r.Context(), workspaceID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

func (h *Handler) Createdocument(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkspaceID  uuid.UUID `json:"workspace_id"`
		FileName     string    `json:"file_name"`
		FileKey      string    `json:"file_key"`
		DocumentType string    `json:"document_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	ws, err := h.svc.Createdocument(r.Context(), body.WorkspaceID, body.FileName, body.FileKey, body.DocumentType)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, ws)
}

func (h *Handler) Updatedocument(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var body struct {
		FileName     string `json:"file_name"`
		DocumentType string `json:"document_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	ws, err := h.svc.Updatedocument(r.Context(), id, body.FileName, body.DocumentType)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

func (h *Handler) Deletedocument(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.svc.Deletedocument(r.Context(), id); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// Error Logging
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, ErrInvalidInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
