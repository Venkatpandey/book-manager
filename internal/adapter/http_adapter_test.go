//go:build unit

package adapter

import (
	"book-manager/api"
	"book-manager/internal/core"
	"book-manager/internal/core/model"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// test wiring: handler + real in-memory service (no network)
func newServer(t *testing.T) (http.Handler, *core.Service) {
	t.Helper()
	repo := NewBookRepo()
	svc := core.NewService(repo, mockEnrich{})
	logger := slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil))
	h := NewHTTPHandler(svc, logger)

	r := chi.NewRouter()
	api.HandlerFromMux(h, r)
	return r, svc
}

type mockEnrich struct{}

func (mockEnrich) FetchByISBN(_ context.Context, _ string) (book model.EnrichedBook, err error) {
	return model.EnrichedBook{}, nil
}

func TestCreateBook_201(t *testing.T) {
	h, _ := newServer(t)
	b := map[string]any{"title": "My Book"}
	body, _ := json.Marshal(b)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/books", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	resp := w.Result()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	loc := resp.Header.Get("Location")
	require.NotEmpty(t, loc)

	var out api.Book
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.NotEmpty(t, out.Id)
	assert.Equal(t, "My Book", out.Title)
}

func TestGetBook_200_and_404(t *testing.T) {
	h, svc := newServer(t)
	// seed via service for convenience
	title := "Seed"
	b, err := svc.CreateBook(context.Background(), model.CreateBookInput{Title: &title})
	require.NoError(t, err)

	// existing
	r1 := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+b.ID, nil)
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, r1)
	require.Equal(t, http.StatusOK, w1.Code)
	var got api.Book
	require.NoError(t, json.NewDecoder(w1.Body).Decode(&got))
	assert.Equal(t, b.ID, got.Id)

	// missing
	r2 := httptest.NewRequest(http.MethodGet, "/api/v1/books/does-not-exist", nil)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestListBooks_Pagination(t *testing.T) {
	h, svc := newServer(t)
	// seed 3
	for i := 1; i <= 3; i++ {
		title := "B" + string(rune('0'+i))
		_, err := svc.CreateBook(context.Background(), model.CreateBookInput{Title: &title})
		require.NoError(t, err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/books?page=1&page_size=2", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	var out api.PaginatedBooks
	require.NoError(t, json.NewDecoder(w.Body).Decode(&out))
	assert.Equal(t, 1, out.Page)
	assert.Equal(t, 2, out.PageSize)
	assert.Equal(t, 3, out.Total)
	assert.Len(t, out.Data, 2)
}

func TestDeleteBook_204_then_404(t *testing.T) {
	h, svc := newServer(t)
	title := "Temp"
	b, err := svc.CreateBook(context.Background(), model.CreateBookInput{Title: &title})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/books/"+b.ID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)

	r2 := httptest.NewRequest(http.MethodGet, "/api/v1/books/"+b.ID, nil)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)
	assert.Equal(t, http.StatusNotFound, w2.Code)
}

func TestCreateBook_Validation400(t *testing.T) {
	h, _ := newServer(t)
	// empty body → invalid JSON or missing title
	r := httptest.NewRequest(http.MethodPost, "/api/v1/books", bytes.NewReader([]byte(`{}`)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
