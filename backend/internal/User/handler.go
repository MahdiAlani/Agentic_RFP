package user

import "net/http"

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}
