package adapter

import (
	"book-manager/api"
	"book-manager/internal/core/model"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

type BookService interface {
	CreateBook(ctx context.Context, in model.CreateBookInput) (model.Book, error)
	ListBooks(ctx context.Context, q model.ListQuery) (model.Page[model.Book], error)
	GetBook(ctx context.Context, id string) (model.Book, error)
	DeleteBook(ctx context.Context, id string) error
}

type HTTPHandler struct {
	Svc BookService
	log *slog.Logger
}

func NewHTTPHandler(svc BookService, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{Svc: svc, log: logger}
}

func (h *HTTPHandler) CreateBook(w http.ResponseWriter, r *http.Request, _ api.CreateBookParams) {
	q := r.URL.Query()
	enrich := q.Get("enrich") == "true"
	require := q.Get("require_enrichment") == "true"

	var in api.BookCreate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION", "invalid JSON body", map[string]any{"cause": err.Error()})
		h.log.With("error", err).Info("invalid JSON body")
		return
	}
	din := toCreateInput(in, enrich, require)
	b, err := h.Svc.CreateBook(r.Context(), din)
	if err != nil {
		status, code := mapSvcErr(err)
		writeErr(w, status, code, err.Error(), nil)
		h.log.With("error", err).Info("create book failed")
		return
	}
	out := fromDomainBook(b)
	w.Header().Set("Location", "/api/v1/books/"+b.ID)
	h.log.Info("create request processed", "book-id", out.Id)
	writeJSON(w, http.StatusCreated, out)
}

func (h *HTTPHandler) ListBooks(w http.ResponseWriter, r *http.Request, p api.ListBooksParams) {
	q := toListQuery(p)
	page, err := h.Svc.ListBooks(r.Context(), q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error(), nil)
		h.log.With("error", err).Info("list books failed")
		return
	}
	writeJSON(w, http.StatusOK, fromDomainPage(page))
}

func (h *HTTPHandler) GetBookById(w http.ResponseWriter, r *http.Request, id string) {
	b, err := h.Svc.GetBook(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "book not found", nil)
		h.log.With("error", err).Info("get book failed")
		return
	}
	writeJSON(w, http.StatusOK, fromDomainBook(b))
}

func (h *HTTPHandler) DeleteBookById(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.Svc.DeleteBook(r.Context(), id); err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "book not found", nil)
		h.log.With("error", err).Info("delete book failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mappers
func toCreateInput(in api.BookCreate, enrich, require bool) model.CreateBookInput {
	var title *string
	if in.Title != "" {
		title = &in.Title
	}
	out := model.CreateBookInput{
		ISBN:              in.Isbn,
		Title:             title,
		Subtitle:          in.Subtitle,
		PublishedYear:     in.PublishedYear,
		PageCount:         in.PageCount,
		CoverURL:          in.CoverUrl,
		Enrich:            enrich,
		RequireEnrichment: require,
	}
	if in.Tags != nil {
		out.Tags = *in.Tags
	}
	if in.Authors != nil {
		out.Authors = append([]string(nil), *in.Authors...)
	}

	return out
}

func toListQuery(p api.ListBooksParams) model.ListQuery {
	q := model.ListQuery{Page: 1, PageSize: 20}
	if p.Page != nil {
		q.Page = *p.Page
	}
	if p.PageSize != nil {
		q.PageSize = *p.PageSize
	}
	q.Q = p.Q
	q.Author = p.Author
	q.Tag = p.Tag
	q.Year = p.Year
	if p.Sort != nil {
		parts := strings.Split(*p.Sort, ",")
		for _, s := range parts {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			sk := model.SortKey{Field: s}
			if strings.HasPrefix(s, "-") {
				sk.Field = s[1:]
				sk.Desc = true
			}
			q.Sort = append(q.Sort, sk)
		}
	}
	return q
}

func fromDomainBook(b model.Book) api.Book {
	src := sourceFromDomain(b.Enrichment.Source)
	looked := strPtrOrNil(b.Enrichment.LookedUpISBN)
	status := statusFromDomain(b.Enrichment.Status)

	return api.Book{
		Id:            b.ID,
		Isbn:          b.ISBN,
		Title:         b.Title,
		Subtitle:      b.Subtitle,
		PublishedYear: b.PublishedYear,
		PageCount:     b.PageCount,
		CoverUrl:      b.CoverURL,
		Tags:          &b.Tags,
		Authors:       toAuthorSummaries(b.Authors),
		Enrichment: &api.EnrichmentMeta{
			Attempted:    b.Enrichment.Attempted,
			Source:       src,
			Status:       status,
			LookedUpIsbn: looked,
		},
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func sourceFromDomain(src string) *api.EnrichmentMetaSource {
	if src == "" {
		return nil
	}
	// Map known sources; default to nil if unknown
	switch strings.ToLower(src) {
	case "openlibrary":
		v := api.Openlibrary
		return &v
	default:
		return nil
	}
}

func statusFromDomain(st model.EnrichmentStatus) api.EnrichmentMetaStatus {
	switch strings.ToLower(string(st)) {
	case "ok":
		return api.Ok
	case "partial":
		return api.Partial
	case "not_requested", "":
		fallthrough
	default:
		return api.NotRequested
	}
}

func toAuthorSummaries(names []string) []api.AuthorSummary {
	out := make([]api.AuthorSummary, 0, len(names))
	for _, n := range names {
		out = append(out, api.AuthorSummary{Id: "", Name: n})
	}
	return out
}

func fromDomainPage(p model.Page[model.Book]) api.PaginatedBooks {
	out := api.PaginatedBooks{Page: p.Page, PageSize: p.PageSize, Total: p.Total}
	for _, b := range p.Data {
		bb := fromDomainBook(b)
		out.Data = append(out.Data, bb)
	}
	return out
}

type errBody struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details,omitempty"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string, det map[string]any) {
	var e errBody
	e.Error.Code = code
	e.Error.Message = msg
	e.Error.Details = det
	writeJSON(w, status, e)
}

func mapSvcErr(err error) (int, string) {
	switch {
	case errors.Is(err, model.ErrValidation):
		return http.StatusBadRequest, "VALIDATION"
	case errors.Is(err, model.ErrConflict):
		return http.StatusConflict, "CONFLICT"
	case errors.Is(err, model.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, model.ErrUpstream):
		return http.StatusBadGateway, "UPSTREAM"
	default:
		return http.StatusInternalServerError, "INTERNAL"
	}
}
