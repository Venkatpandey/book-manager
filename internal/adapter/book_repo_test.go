//go:build unit

package adapter

import (
	"book-manager/internal/core/model"
	"book-manager/pkg/util"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndGet(t *testing.T) {
	r := NewBookRepo()
	b := model.Book{ID: "b1", Title: "T1", ISBN: util.GetPtr("978-0-12-345678-9"), CreatedAt: time.Unix(1000, 0)}

	ctx := context.Background()
	created, err := r.Create(ctx, b)
	require.NoError(t, err)
	assert.Equal(t, "b1", created.ID)
	assert.Equal(t, "T1", created.Title)

	got, err := r.GetByID(ctx, "b1")
	require.NoError(t, err)
	assert.Equal(t, "T1", got.Title)

	got2, err := r.GetByISBN(ctx, "9780123456789")
	require.NoError(t, err)
	assert.Equal(t, "b1", got2.ID)
}

func TestDuplicateISBN(t *testing.T) {
	r := NewBookRepo()
	b1 := model.Book{ID: "b1", Title: "A", ISBN: util.GetPtr("978-1-23-000000-0")}
	b2 := model.Book{ID: "b2", Title: "B", ISBN: util.GetPtr("9781230000000")}

	ctx := context.Background()
	_, err := r.Create(ctx, b1)
	require.NoError(t, err)
	_, err = r.Create(ctx, b2)
	assert.ErrorIs(t, err, errConflict)
}

func TestListFiltersAndPagination(t *testing.T) {
	r := NewBookRepo()
	mk := func(id, title string, year int, authors []string, tags []string, created int64) model.Book {
		return model.Book{ID: id, Title: title, PublishedYear: util.GetPtr(year), Authors: authors, Tags: tags, CreatedAt: time.Unix(created, 0)}
	}
	seed := []model.Book{
		mk("b1", "Go in Action", 2015, []string{"William"}, []string{"go"}, 1000),
		mk("b2", "The Go Programming Language", 2016, []string{"Alan"}, []string{"go", "lang"}, 1010),
		mk("b3", "Clean Architecture", 2017, []string{"Robert"}, []string{"arch"}, 1020),
		mk("b4", "Domain-Driven Design", 2003, []string{"Eric"}, []string{"ddd"}, 1030),
	}
	ctx := context.Background()
	for _, b := range seed {
		_, err := r.Create(ctx, b)
		require.NoError(t, err)
	}

	qq := "go"
	page, err := r.List(ctx, model.ListQuery{Q: &qq, Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, page.Data, 2)

	a := "alan"
	page, err = r.List(ctx, model.ListQuery{Author: &a, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
	assert.Equal(t, "b2", page.Data[0].ID)

	tag := "ddd"
	page, err = r.List(ctx, model.ListQuery{Tag: &tag, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
	assert.Equal(t, "b4", page.Data[0].ID)

	page, err = r.List(ctx, model.ListQuery{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 4, page.Total)
	assert.Len(t, page.Data, 2)

	page2, err := r.List(ctx, model.ListQuery{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Len(t, page2.Data, 2)
}

func TestDelete(t *testing.T) {
	r := NewBookRepo()
	ctx := context.Background()
	_, err := r.Create(ctx, model.Book{ID: "b1", Title: "T"})
	require.NoError(t, err)
	assert.NoError(t, r.Delete(ctx, "b1"))
	_, err = r.GetByID(ctx, "b1")
	assert.ErrorIs(t, err, errNotFound)
}
