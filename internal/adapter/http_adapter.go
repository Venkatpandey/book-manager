package adapter

import (
	"book-manager/api"
	"encoding/json"
	"log/slog"
	"net/http"
)

type BookService interface {
	CreateBook(ctx any, input any) (any, error)
	ListBooks(ctx any, query any) (any, error)
	GetBook(ctx any, id string) (any, error)
	DeleteBook(ctx any, id string) error
}

type Handler struct {
	Svc BookService
	log *slog.Logger
}

func NewHandler(svc BookService, logger *slog.Logger) *Handler {
	return &Handler{Svc: svc, log: logger}
}

type httpError struct {
	Error struct {
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Details map[string]interface{} `json:"details,omitempty"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string, details map[string]interface{}) {
	e := httpError{}
	e.Error.Code = code
	e.Error.Message = msg
	e.Error.Details = details
	writeJSON(w, status, e)
}

func (h *Handler) CreateBook(w http.ResponseWriter, r *http.Request, params api.CreateBookParams) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "CreateBook not implemented", nil)
}

func (h *Handler) ListBooks(w http.ResponseWriter, r *http.Request, params api.ListBooksParams) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "ListBooks not implemented", nil)
}

func (h *Handler) GetBookById(w http.ResponseWriter, r *http.Request, id string) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "GetBookById not implemented", nil)
}

func (h *Handler) DeleteBookById(w http.ResponseWriter, r *http.Request, id string) {
	w.WriteHeader(http.StatusNotImplemented)
}
