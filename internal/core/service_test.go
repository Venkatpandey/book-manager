package core

import (
	"book-manager/internal/adapter"
	"book-manager/internal/core/model"
	"book-manager/pkg/util"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate_NoEnrich(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})
	title := "My Book"
	out, err := svc.CreateBook(context.Background(), model.CreateBookInput{Title: &title})
	require.NoError(t, err)
	assert.Equal(t, "My Book", out.Title)
	assert.False(t, out.Enrichment.Attempted)
}

func TestCreate_EnrichHit_Merges(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: true})
	isbn := "9780134494166"
	out, err := svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: &isbn, Enrich: true})
	require.NoError(t, err)
	assert.True(t, out.Enrichment.Attempted)
	assert.Equal(t, model.EnrichmentOK, out.Enrichment.Status)
	assert.Equal(t, "Clean Architecture", out.Title)
	assert.NotZero(t, out.CreatedAt)
}

func TestCreate_EnrichMiss_RequireFalse_AllowsPartial(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})
	isbn := "9780134494166"
	title := "Fallback Title"
	out, err := svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: &isbn, Enrich: true, RequireEnrichment: false, Title: &title})
	require.NoError(t, err)
	assert.Equal(t, model.EnrichmentPartial, out.Enrichment.Status)
	assert.Equal(t, "Fallback Title", out.Title)
}

func TestCreate_EnrichMiss_RequireTrue_Fails(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})
	isbn := "9780134494166"
	_, err := svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: &isbn, Enrich: true, RequireEnrichment: true})
	require.Error(t, err)
	assert.Equal(t, ErrUpstream, err)
}

func TestDuplicateISBN_Fails(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})
	isbn := "9780000000000"
	_, err := svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: &isbn, Title: util.GetPtr("T")})
	require.NoError(t, err)
	_, err = svc.CreateBook(context.Background(), model.CreateBookInput{ISBN: &isbn, Title: util.GetPtr("T")})
	require.Error(t, err)
}

func TestGetAndDelete(t *testing.T) {
	repo := adapter.NewBookRepo()
	svc := NewService(repo, mockEnrich{hit: false})
	title := "X"
	b, err := svc.CreateBook(context.Background(), model.CreateBookInput{Title: &title})
	require.NoError(t, err)
	got, err := svc.GetBook(context.Background(), b.ID)
	require.NoError(t, err)
	assert.Equal(t, b.ID, got.ID)
	require.NoError(t, svc.DeleteBook(context.Background(), b.ID))
	_, err = svc.GetBook(context.Background(), b.ID)
	require.Error(t, err)
}

type mockEnrich struct{ hit bool }

func (f mockEnrich) FetchByISBN(ctx context.Context, isbn string) (model.EnrichedBook, error) {
	if !f.hit {
		return model.EnrichedBook{}, errors.New("miss")
	}
	y := 2017
	p := 432
	return model.EnrichedBook{
		Title: util.GetPtr("Clean Architecture"), PublishedYear: &y, PageCount: &p, Authors: []string{"Robert C. Martin"}}, nil
}
