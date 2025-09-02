//go:build unit

package core

import (
	"book-manager/internal/adapter"
	"book-manager/internal/core/model"
	"book-manager/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate_NoEnrich(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})
	out, err := svc.CreateBook(context.Background(), model.CreateBookInput{Title: util.GetPtr("My Book")})
	require.NoError(t, err)
	assert.Equal(t, "My Book", out.Title)
	assert.False(t, out.Enrichment.Attempted)
}

func TestCreate_EnrichHit_Merges(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: true})

	out, err := svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: util.GetPtr("9780134494166"), Enrich: true})
	require.NoError(t, err)
	assert.True(t, out.Enrichment.Attempted)
	assert.Equal(t, model.EnrichmentOK, out.Enrichment.Status)
	assert.Equal(t, "Clean Architecture", out.Title)
	assert.NotZero(t, out.CreatedAt)
}

func TestCreate_EnrichMiss_RequireFalse_AllowsPartial(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})

	out, err := svc.CreateBook(context.Background(), model.CreateBookInput{
		ISBN: util.GetPtr("9780134494166"), Enrich: true, RequireEnrichment: false, Title: util.GetPtr("Fallback Title")})
	require.NoError(t, err)
	assert.Equal(t, model.EnrichmentPartial, out.Enrichment.Status)
	assert.Equal(t, "Fallback Title", out.Title)
}

func TestCreate_EnrichMiss_RequireTrue_Fails(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})

	_, err := svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: util.GetPtr("9780134494166"), Enrich: true, RequireEnrichment: true})
	require.Error(t, err)
	assert.Equal(t, model.ErrUpstream, err)
}

func TestDuplicateISBN_Fails(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})
	isbn := "9780000000000"
	ctx := context.Background()
	_, err := svc.CreateBook(ctx, model.CreateBookInput{ISBN: util.GetPtr(isbn), Title: util.GetPtr("T")})
	require.NoError(t, err)
	_, err = svc.CreateBook(ctx, model.CreateBookInput{ISBN: util.GetPtr(isbn), Title: util.GetPtr("T")})
	require.Error(t, err)
}

func TestGetAndDelete(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})
	ctx := context.Background()
	b, err := svc.CreateBook(ctx, model.CreateBookInput{Title: util.GetPtr("X")})
	require.NoError(t, err)
	got, err := svc.GetBook(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, b.ID, got.ID)
	require.NoError(t, svc.DeleteBook(ctx, b.ID))
	_, err = svc.GetBook(ctx, b.ID)
	require.Error(t, err)
}

func TestService_Create_WithEnrichment_OK(t *testing.T) {
	// Mock Open Library server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/isbn/9780134494166.json" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"title":           "Clean Architecture",
				"number_of_pages": 400,
				"publish_date":    "2020",
				"covers":          []int{5555},
				"authors":         []map[string]any{{"name": "Robert C. Martin"}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	repo := adapter.NewBookRepo()
	client := adapter.NewOpenLibraryClient(ts.URL, 1, http.DefaultClient)
	svc := NewService(repo, client)

	out, err := svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: util.GetPtr("9780134494166"), Enrich: true})
	require.NoError(t, err)
	assert.Equal(t, "Clean Architecture", out.Title)
	assert.Equal(t, "ok", string(out.Enrichment.Status))
	require.NotNil(t, out.PageCount)
	assert.Equal(t, 400, *out.PageCount)
	require.NotNil(t, out.PublishedYear)
	assert.Equal(t, 2020, *out.PublishedYear)
}

func TestService_Create_WithEnrichment_RequireTrue_404Fails(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	repo := adapter.NewBookRepo()
	client := adapter.NewOpenLibraryClient(ts.URL, 1, http.DefaultClient)
	svc := NewService(repo, client)

	_, err := svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: util.GetPtr("0000000000"), Enrich: true, RequireEnrichment: true})
	require.Error(t, err)
}

type mockEnrich struct{ hit bool }

func (f mockEnrich) FetchByISBN(ctx context.Context, isbn string) (model.EnrichedBook, error) {
	if !f.hit {
		return model.EnrichedBook{}, errors.New("miss")
	}

	return model.EnrichedBook{
		Title: util.GetPtr("Clean Architecture"), PublishedYear: util.GetPtr(2017), PageCount: util.GetPtr(432), Authors: []string{"Robert C. Martin"}}, nil
}
