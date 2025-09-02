package adapter

import (
	"book-manager/internal/core/model"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type OpenLibraryClient struct {
	BaseURL string
	Client  *http.Client
	Retry   int
}

func NewOpenLibraryClient(baseURL string, retry int, httpClient *http.Client) *OpenLibraryClient {
	if baseURL == "" {
		baseURL = "https://openlibrary.org"
	}
	if retry < 0 {
		retry = 0
	}
	return &OpenLibraryClient{
		BaseURL: baseURL,
		Client:  httpClient,
		Retry:   retry,
	}
}

func (c *OpenLibraryClient) FetchByISBN(ctx context.Context, isbn string) (model.EnrichedBook, error) {
	url := fmt.Sprintf("%s/isbn/%s.json", c.BaseURL, isbn)

	var lastErr error
	attempts := c.Retry + 1
	for i := 0; i < attempts; i++ {
		eb, err := c.fetchOnce(ctx, url)
		if err == nil {
			return eb, nil
		}
		// 404 is final: not found
		if errors.Is(err, errNotFound) {
			return model.EnrichedBook{}, err
		}
		lastErr = err
		// simple backoff
		if i < attempts-1 {
			select {
			case <-time.After(time.Duration(150*(i+1)) * time.Millisecond):
			case <-ctx.Done():
				return model.EnrichedBook{}, ctx.Err()
			}
		}
	}
	return model.EnrichedBook{}, lastErr
}

func (c *OpenLibraryClient) fetchOnce(ctx context.Context, url string) (model.EnrichedBook, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return model.EnrichedBook{}, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return model.EnrichedBook{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return model.EnrichedBook{}, errNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return model.EnrichedBook{}, fmt.Errorf("openlibrary: status %d: %s", resp.StatusCode, string(b))
	}

	var ob openLibBook
	if err := json.NewDecoder(resp.Body).Decode(&ob); err != nil {
		return model.EnrichedBook{}, err
	}

	return mapToEnriched(ob), nil
}

type openLibBook struct {
	Title         *string     `json:"title"`
	Subtitle      *string     `json:"subtitle"`
	NumberOfPages *int        `json:"number_of_pages"`
	PublishDate   *string     `json:"publish_date"` // e.g. "2017"
	Covers        []int       `json:"covers"`
	Authors       []olbAuthor `json:"authors"`
}

type olbAuthor struct {
	Name *string `json:"name"` // sometimes present directly
	// If only a key like "/authors/OL123A" appears without name, we skip name to keep it simple.
}

func mapToEnriched(ob openLibBook) model.EnrichedBook {
	var year *int
	if ob.PublishDate != nil {
		// best effort: parse leading 4-digit year
		if len(*ob.PublishDate) >= 4 {
			if y, err := parseYear(*ob.PublishDate); err == nil {
				year = &y
			}
		}
	}

	var cover *string
	if len(ob.Covers) > 0 {
		u := fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", ob.Covers[0])
		cover = &u
	}

	authors := make([]string, 0, len(ob.Authors))
	for _, a := range ob.Authors {
		if a.Name != nil && *a.Name != "" {
			authors = append(authors, *a.Name)
		}
	}

	return model.EnrichedBook{
		Title:         ob.Title,
		Subtitle:      ob.Subtitle,
		PublishedYear: year,
		PageCount:     ob.NumberOfPages,
		CoverURL:      cover,
		Authors:       authors,
	}
}

var yearRe = regexp.MustCompile(`(\d{4})`)

// parseYear extracts the first 4-digit sequence from the string.
// Returns error if no 4-digit number found.
func parseYear(s string) (int, error) {
	match := yearRe.FindString(s)
	if match == "" {
		return 0, fmt.Errorf("no year in %q", s)
	}
	return strconv.Atoi(match)
}
