package model

import (
	"errors"
	"time"
)

// All core models live here together for simplicity.

type EnrichmentStatus string

const (
	EnrichmentNotRequested EnrichmentStatus = "not_requested"
	EnrichmentOK           EnrichmentStatus = "ok"
	EnrichmentPartial      EnrichmentStatus = "partial"
)

var (
	ErrValidation = errors.New("validation")
	ErrConflict   = errors.New("conflict")
	ErrNotFound   = errors.New("not_found")
	ErrUpstream   = errors.New("upstream")
)

type EnrichmentMeta struct {
	Attempted    bool
	Source       string // e.g., "openlibrary"
	Status       EnrichmentStatus
	LookedUpISBN string
}

type Book struct {
	ID            string
	ISBN          *string
	Title         string
	Subtitle      *string
	PublishedYear *int
	PageCount     *int
	CoverURL      *string
	Tags          []string
	Authors       []string // names only; no author entity
	Enrichment    EnrichmentMeta
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Page[T any] struct {
	Data     []T
	Page     int
	PageSize int
	Total    int
}

type SortKey struct {
	Field string // title | published_year | created_at | updated_at
	Desc  bool
}

type ListQuery struct {
	Q        *string // search in title/subtitle
	Author   *string // contains, case-insensitive
	Year     *int
	Tag      *string // exact
	Sort     []SortKey
	Page     int
	PageSize int
}

type EnrichedBook struct {
	Title         *string
	Subtitle      *string
	PublishedYear *int
	PageCount     *int
	CoverURL      *string
	Authors       []string
}

type CreateBookInput struct {
	ISBN              *string
	Title             *string
	Subtitle          *string
	PublishedYear     *int
	PageCount         *int
	CoverURL          *string
	Tags              []string
	Authors           []string
	Enrich            bool
	RequireEnrichment bool
}
