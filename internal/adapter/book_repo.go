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

func (r *BookRepo) Create(_ context.Context, b model.Book) (model.Book, error) {
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

func (r *BookRepo) GetByID(_ context.Context, id string) (model.Book, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.byID[id]
	if !ok {
		return model.Book{}, errNotFound
	}
	return copyBook(b), nil
}

func (r *BookRepo) GetByISBN(_ context.Context, isbn string) (model.Book, error) {
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

// List returns a paginated slice of books matching the query.
// The flow is:
//
//  1. Snapshot all books from the in-memory store (thread-safe copy).
//  2. Apply filters (title/subtitle full-text, author, tag, year, etc.).
//  3. Sort the filtered books according to the provided sort keys
//     (supports multi-field, ASC/DESC). Defaults to created_at DESC.
//  4. Apply pagination (page / page_size).
func (r *BookRepo) List(_ context.Context, q model.ListQuery) (model.Page[model.Book], error) {
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

func (r *BookRepo) Delete(_ context.Context, id string) error {
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

// matchFilters checks whether a book matches the given query filters.
func matchFilters(b model.Book, q model.ListQuery) bool {
	// Full-text search: title or subtitle contains the query (case-insensitive)
	// q: title or subtitle contains (case-insensitive)
	if q.Q != nil {
		needle := strings.ToLower(*q.Q)
		if !strings.Contains(strings.ToLower(b.Title), needle) {
			sub := ""
			if b.Subtitle != nil {
				sub = strings.ToLower(*b.Subtitle)
			}
			if !strings.Contains(sub, needle) {
				return false
			}
		}
	}

	// author: any author contains (case-insensitive)
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

	// tag: exact match
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

	// year: exact
	if q.Year != nil {
		if b.PublishedYear == nil || *b.PublishedYear != *q.Year {
			return false
		}
	}
	return true
}

// sortBooks sorts books in-place by the provided sort keys.
// Supports multiple fields (title, published_year, created_at, updated_at).
// Falls back to ID for stability.
func sortBooks(bs []model.Book, keys []model.SortKey) {
	if len(keys) == 0 {
		// default: created_at desc
		sort.Slice(bs, func(i, j int) bool { return bs[i].CreatedAt.After(bs[j].CreatedAt) })
		return
	}

	// Apply multi-key sorting, respecting ASC/DESC
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
				ai, bi := bs[i].PublishedYear, bs[j].PublishedYear
				switch {
				case ai == nil && bi == nil:
					// equal, continue to next key
				case ai == nil:
					if k.Desc {
						return false
					} // nil < val
					return true
				case bi == nil:
					if k.Desc {
						return true
					}
					return false
				default:
					if *ai != *bi {
						if k.Desc {
							return *ai > *bi
						}
						return *ai < *bi
					}
				}
			case "created_at":
				if !bs[i].CreatedAt.Equal(bs[j].CreatedAt) {
					if k.Desc {
						return bs[i].CreatedAt.After(bs[j].CreatedAt)
					}
					return bs[i].CreatedAt.Before(bs[j].CreatedAt)
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
		// sort by ID for deterministic ordering
		return bs[i].ID < bs[j].ID
	})
}
