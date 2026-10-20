package answers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

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
	mux.HandleFunc("POST /answers", h.CreateAnswer)
	mux.HandleFunc("GET /answers/{id}", h.GetAnswer)
	mux.HandleFunc("PATCH /answers/{id}", h.UpdateAnswer)
	mux.HandleFunc("POST /answers/{id}/regenerate", h.RegenerateAnswer)
	mux.HandleFunc("POST /answers/{id}/approve", h.ApproveAnswer)
	mux.HandleFunc("GET /workspaces/{workspaceID}/answers", h.ListAnswers)
	mux.HandleFunc("GET /projects/{projectID}/answers", h.ListProjectAnswers)
	mux.HandleFunc("POST /projects/{projectID}/answers/fill", h.FillProject)
	mux.HandleFunc("POST /projects/{projectID}/answers/extract", h.ExtractQuestions)
	mux.HandleFunc("GET /projects/{projectID}/answers/export", h.ExportProject)
	mux.HandleFunc("DELETE /answers/{id}", h.DeleteAnswer)
}

func (h *Handler) ListProjectAnswers(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	as, err := h.svc.ListProjectAnswers(r.Context(), projectID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, as)
}

// ExportProject downloads the RFP response as a Word document.
func (h *Handler) ExportProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	name, as, err := h.svc.ExportProject(r.Context(), projectID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	// Build into memory first so a failure can still return a clean 500.
	var buf bytes.Buffer
	if err := buildResponse(name, as, time.Now()).Write(&buf); err != nil {
		writeError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", docxContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, exportFilename(name)))
	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("%s %s: write export: %v", r.Method, r.URL.Path, err)
	}
}

func (h *Handler) UpdateAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var body struct {
		AnswerText string `json:"answer_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	a, err := h.svc.UpdateAnswerText(r.Context(), id, body.AnswerText)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) RegenerateAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	a, err := h.svc.RegenerateAnswer(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) ApproveAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	a, err := h.svc.ApproveAnswer(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) FillProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	queued, err := h.svc.FillProject(r.Context(), projectID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusAccepted, struct {
		Queued int `json:"queued"`
	}{Queued: queued})
}

func (h *Handler) ExtractQuestions(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	var body struct {
		WorkspaceID uuid.UUID `json:"workspace_id"`
		DocumentID  uuid.UUID `json:"document_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := h.svc.ExtractQuestions(r.Context(), body.WorkspaceID, projectID, body.DocumentID); err != nil {
		writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) CreateAnswer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkspaceID uuid.UUID  `json:"workspace_id"`
		ProjectID   *uuid.UUID `json:"project_id"`
		DocumentID  *uuid.UUID `json:"document_id"`
		Question    string     `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	a, err := h.svc.CreateAnswer(r.Context(), body.WorkspaceID, body.ProjectID, body.DocumentID, body.Question)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, a)
}

func (h *Handler) GetAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	a, err := h.svc.GetAnswer(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) ListAnswers(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := uuid.Parse(r.PathValue("workspaceID"))
	if err != nil {
		http.Error(w, "invalid workspace id", http.StatusBadRequest)
		return
	}

	as, err := h.svc.ListAnswers(r.Context(), workspaceID)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, as)
}

func (h *Handler) DeleteAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.svc.DeleteAnswer(r.Context(), id); err != nil {
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
	default:
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
