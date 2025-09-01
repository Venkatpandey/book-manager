package adapter

import (
	"book-manager/internal/core/model"
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
)

var (
	errNotFound = errors.New("not found")
	errConflict = errors.New("conflict")
)

type BookRepo struct {
	mu     sync.RWMutex
	byID   map[string]model.Book // id -> Book
	byISBN map[string]string     // normalized ISBN -> id
}

func NewBookRepo() *BookRepo {
	return &BookRepo{byID: make(map[string]model.Book), byISBN: make(map[string]string)}
}

func (r *BookRepo) Create(ctx context.Context, b model.Book) (model.Book, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if b.ID == "" {
		return model.Book{}, errConflict
	}
	if _, ok := r.byID[b.ID]; ok {
		return model.Book{}, errConflict
	}

	if b.ISBN != nil {
		key := normalizeISBN(*b.ISBN)
		if key != "" {
			if _, exists := r.byISBN[key]; exists {
				return model.Book{}, errConflict
			}
			r.byISBN[key] = b.ID
		}
	}
	r.byID[b.ID] = copyBook(b)
	return copyBook(b), nil
}

func (r *BookRepo) GetByID(ctx context.Context, id string) (model.Book, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.byID[id]
	if !ok {
		return model.Book{}, errNotFound
	}
	return copyBook(b), nil
}

func (r *BookRepo) GetByISBN(ctx context.Context, isbn string) (model.Book, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := normalizeISBN(isbn)
	id, ok := r.byISBN[key]
	if !ok {
		return model.Book{}, errNotFound
	}
	b, ok := r.byID[id]
	if !ok {
		return model.Book{}, errNotFound
	}
	return copyBook(b), nil
}

func (r *BookRepo) List(ctx context.Context, q model.ListQuery) (model.Page[model.Book], error) {
	r.mu.RLock()
	// snapshot ids to avoid holding lock during sort
	items := make([]model.Book, 0, len(r.byID))
	for _, b := range r.byID {
		items = append(items, copyBook(b))
	}
	r.mu.RUnlock()

	// filters
	out := items[:0]
	for _, b := range items {
		if !matchFilters(b, q) {
			continue
		}
		out = append(out, b)
	}

	// sort
	sortBooks(out, q.Sort)

	// pagination
	page := q.Page
	if page < 1 {
		page = 1
	}
	size := q.PageSize
	if size < 1 {
		size = 20
	}
	total := len(out)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	paged := make([]model.Book, end-start)
	copy(paged, out[start:end])

	return model.Page[model.Book]{Data: paged, Page: page, PageSize: size, Total: total}, nil
}

func (r *BookRepo) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.byID[id]
	if !ok {
		return errNotFound
	}
	if b.ISBN != nil {
		key := normalizeISBN(*b.ISBN)
		delete(r.byISBN, key)
	}
	delete(r.byID, id)
	return nil
}

// helpers

func copyBook(b model.Book) model.Book {
	b.Tags = append([]string(nil), b.Tags...)
	b.Authors = append([]string(nil), b.Authors...)
	return b
}

func normalizeISBN(s string) string {
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func matchFilters(b model.Book, q model.ListQuery) bool {
	if q.Q != nil {
		needle := strings.ToLower(*q.Q)
		t := strings.ToLower(b.Title)
		sub := ""
		if b.Subtitle != nil {
			sub = strings.ToLower(*b.Subtitle)
		}
		if !strings.Contains(t, needle) && !strings.Contains(sub, needle) {
			return false
		}
	}
	if q.Author != nil {
		needle := strings.ToLower(*q.Author)
		found := false
		for _, a := range b.Authors {
			if strings.Contains(strings.ToLower(a), needle) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q.Tag != nil {
		found := false
		for _, t := range b.Tags {
			if t == *q.Tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if q.Year != nil {
		if b.PublishedYear == nil || *b.PublishedYear != *q.Year {
			return false
		}
	}
	return true
}

func cmpPtrInt(a, b *int) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	if *a < *b {
		return -1
	}
	if *a > *b {
		return 1
	}
	return 0
}

func cmpTime(a, b model.Book) int {
	if a.CreatedAt.Before(b.CreatedAt) {
		return -1
	}
	if a.CreatedAt.After(b.CreatedAt) {
		return 1
	}
	return 0
}

func sortBooks(bs []model.Book, keys []model.SortKey) {
	if len(keys) == 0 {
		// default: created_at desc
		sort.Slice(bs, func(i, j int) bool { return bs[i].CreatedAt.After(bs[j].CreatedAt) })
		return
	}
	sort.SliceStable(bs, func(i, j int) bool {
		for _, k := range keys {
			switch k.Field {
			case "title":
				if bs[i].Title != bs[j].Title {
					if k.Desc {
						return bs[i].Title > bs[j].Title
					}
					return bs[i].Title < bs[j].Title
				}
			case "published_year":
				c := cmpPtrInt(bs[i].PublishedYear, bs[j].PublishedYear)
				if c != 0 {
					if k.Desc {
						return c > 0
					}
					return c < 0
				}
			case "created_at":
				c := cmpTime(bs[i], bs[j])
				if c != 0 {
					if k.Desc {
						return c > 0
					}
					return c < 0
				}
			case "updated_at":
				if !bs[i].UpdatedAt.Equal(bs[j].UpdatedAt) {
					if k.Desc {
						return bs[i].UpdatedAt.After(bs[j].UpdatedAt)
					}
					return bs[i].UpdatedAt.Before(bs[j].UpdatedAt)
				}
			}
		}
		// tie-breaker by ID for stability
		return bs[i].ID < bs[j].ID
	})
}
