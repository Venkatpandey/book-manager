//go:build integration

package adapter

import (
	"book-manager/pkg/http_client"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenLibrary_Live(t *testing.T) {
	c := NewOpenLibraryClient("https://openlibrary.org", 2, http_client.CreateHTTPClient())
	eb, err := c.FetchByISBN(context.Background(), "9780134494166")
	require.NoError(t, err)
	require.NotNil(t, eb.Title)
}
