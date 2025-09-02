package core

import (
	"book-manager/internal/core/model"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type BookRepository interface {
	Create(ctx context.Context, b model.Book) (model.Book, error)
	GetByID(ctx context.Context, id string) (model.Book, error)
	GetByISBN(ctx context.Context, isbn string) (model.Book, error)
	List(ctx context.Context, q model.ListQuery) (model.Page[model.Book], error)
	Delete(ctx context.Context, id string) error
}

type EnrichmentClient interface {
	FetchByISBN(ctx context.Context, isbn string) (model.EnrichedBook, error)
}

var (
	ErrValidation = errors.New("validation")
	ErrConflict   = errors.New("conflict")
	ErrNotFound   = errors.New("not_found")
	ErrUpstream   = errors.New("upstream")
)

type Service struct {
	Repo   BookRepository
	Enrich EnrichmentClient
}

func NewService(repo BookRepository, enrich EnrichmentClient) *Service {
	return &Service{Repo: repo, Enrich: enrich}
}

func (s *Service) CreateBook(ctx context.Context, in model.CreateBookInput) (model.Book, error) {
	// basic validation
	if !in.Enrich || in.ISBN == nil {
		if in.Title == nil || *in.Title == "" {
			return model.Book{}, ErrValidation
		}
	}
	if in.PageCount != nil && *in.PageCount < 1 {
		return model.Book{}, ErrValidation
	}
	if in.PublishedYear != nil {
		y := *in.PublishedYear
		if y < 1450 || y > 3000 {
			return model.Book{}, ErrValidation
		}
	}

	b := model.Book{
		ID:            uuid.NewString(),
		ISBN:          in.ISBN,
		Title:         valueOr(in.Title, ""),
		Subtitle:      in.Subtitle,
		PublishedYear: in.PublishedYear,
		PageCount:     in.PageCount,
		CoverURL:      in.CoverURL,
		Tags:          append([]string(nil), in.Tags...),
		Authors:       append([]string(nil), in.Authors...),
		Enrichment:    model.EnrichmentMeta{Attempted: false, Status: model.EnrichmentNotRequested},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// optional enrichment
	if in.Enrich && in.ISBN != nil && *in.ISBN != "" {
		b.Enrichment.Attempted = true
		b.Enrichment.Source = "openlibrary"
		b.Enrichment.LookedUpISBN = *in.ISBN
		res, err := s.Enrich.FetchByISBN(ctx, *in.ISBN)
		if err != nil {
			if in.RequireEnrichment {
				return model.Book{}, ErrUpstream
			}
			b.Enrichment.Status = model.EnrichmentPartial
		} else {
			merge(&b, res) // fill only missing fields; user wins
			b.Enrichment.Status = model.EnrichmentOK
		}
	}

	// duplicate ISBN protection via repo (GetByISBN) before create
	if b.ISBN != nil && *b.ISBN != "" {
		if _, err := s.Repo.GetByISBN(ctx, *b.ISBN); err == nil {
			return model.Book{}, ErrConflict
		}
	}

	created, err := s.Repo.Create(ctx, b)
	if err != nil {
		// map repo errors if needed
		return model.Book{}, err
	}
	return created, nil
}

func (s *Service) ListBooks(ctx context.Context, q model.ListQuery) (model.Page[model.Book], error) {
	return s.Repo.List(ctx, q)
}

func (s *Service) GetBook(ctx context.Context, id string) (model.Book, error) {
	b, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return model.Book{}, ErrNotFound
	}
	return b, nil
}

func (s *Service) DeleteBook(ctx context.Context, id string) error {
	if err := s.Repo.Delete(ctx, id); err != nil {
		return ErrNotFound
	}
	return nil
}

// helpers
func valueOr(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

func merge(dst *model.Book, e model.EnrichedBook) {
	if dst.Title == "" && e.Title != nil {
		dst.Title = *e.Title
	}
	if dst.Subtitle == nil && e.Subtitle != nil {
		dst.Subtitle = e.Subtitle
	}
	if dst.PublishedYear == nil && e.PublishedYear != nil {
		dst.PublishedYear = e.PublishedYear
	}
	if dst.PageCount == nil && e.PageCount != nil {
		dst.PageCount = e.PageCount
	}
	if dst.CoverURL == nil && e.CoverURL != nil {
		dst.CoverURL = e.CoverURL
	}
	if len(dst.Authors) == 0 && len(e.Authors) > 0 {
		dst.Authors = append([]string(nil), e.Authors...)
	}
}
