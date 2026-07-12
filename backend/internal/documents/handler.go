package documents

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
	mux.HandleFunc("POST /documents", h.CreateDocument)
	mux.HandleFunc("POST /documents/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("GET /documents/{id}", h.GetDocument)
	mux.HandleFunc("GET /documents/{id}/download", h.DownloadURL)
	mux.HandleFunc("GET /workspaces/{workspaceID}/documents", h.ListDocuments)
	mux.HandleFunc("DELETE /documents/{id}", h.DeleteDocument)
}

type createResponse struct {
	*document
	UploadURL string `json:"upload_url"`
}

func (h *Handler) CreateDocument(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkspaceID  uuid.UUID  `json:"workspace_id"`
		ProjectID    *uuid.UUID `json:"project_id"`
		FileName     string     `json:"file_name"`
		DocumentType string     `json:"document_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	doc, uploadURL, err := h.svc.CreateDocument(r.Context(), body.WorkspaceID, body.ProjectID, body.FileName, body.DocumentType)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, createResponse{document: doc, UploadURL: uploadURL})
}

func (h *Handler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	doc, err := h.svc.ConfirmUpload(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	doc, err := h.svc.GetDocument(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (h *Handler) DownloadURL(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	url, err := h.svc.DownloadURL(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, struct {
		DownloadURL string `json:"download_url"`
	}{DownloadURL: url})
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := uuid.Parse(r.PathValue("workspaceID"))
	if err != nil {
		http.Error(w, "invalid workspace id", http.StatusBadRequest)
		return
	}

	docs, err := h.svc.ListDocuments(r.Context(), workspaceID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, docs)
}

func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteDocument(r.Context(), id); err != nil {
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
	case errors.Is(err, ErrNotUploaded):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
