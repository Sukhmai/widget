package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// WikipediaFetcher implements Fetcher for *.wikipedia.org/wiki/ URLs.
// It calls the Wikipedia REST API (no key required), supporting all language editions.
type WikipediaFetcher struct {
	httpClient *http.Client
}

func NewWikipediaFetcher() *WikipediaFetcher {
	return &WikipediaFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch implements Fetcher.
func (f *WikipediaFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	lang, title, err := parseWikipediaURL(resourceURL)
	if err != nil {
		return nil, err
	}

	apiURL := "https://" + lang + ".wikipedia.org/api/rest_v1/page/summary/" + url.PathEscape(title)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("wikipedia: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", identifyingUA)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wikipedia: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("wikipedia: API returned %d: %s", resp.StatusCode, b)
	}

	var data struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Extract     string `json:"extract"`
		Thumbnail   *struct {
			Source string `json:"source"`
		} `json:"thumbnail"`
		ContentURLs *struct {
			Desktop *struct {
				Page string `json:"page"`
			} `json:"desktop"`
		} `json:"content_urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("wikipedia: decode response: %w", err)
	}

	var thumbnailURL string
	if data.Thumbnail != nil {
		thumbnailURL = data.Thumbnail.Source
	}
	var canonicalURL string
	if data.ContentURLs != nil && data.ContentURLs.Desktop != nil {
		canonicalURL = data.ContentURLs.Desktop.Page
	}
	if canonicalURL == "" {
		canonicalURL = resourceURL
	}

	meta := &resourcev1.WikipediaMetadata{
		ThumbnailUrl: thumbnailURL,
		Title:        data.Title,
		Description:  data.Description,
		Extract:      data.Extract,
		WikipediaUrl: canonicalURL,
	}
	return protojson.Marshal(meta)
}

// parseWikipediaURL extracts the language code and article title from a
// Wikipedia URL of the form https://{lang}.wikipedia.org/wiki/{title}.
func parseWikipediaURL(rawURL string) (lang, title string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("wikipedia: parse URL: %w", err)
	}
	// host is e.g. "en.wikipedia.org"
	parts := strings.SplitN(u.Hostname(), ".", 2)
	if len(parts) != 2 || parts[1] != "wikipedia.org" {
		return "", "", fmt.Errorf("wikipedia: unexpected host %q", u.Hostname())
	}
	lang = parts[0]

	// path is e.g. "/wiki/Ada_Lovelace"
	after, ok := strings.CutPrefix(u.Path, "/wiki/")
	if !ok || after == "" {
		return "", "", fmt.Errorf("wikipedia: no article title in path %q", u.Path)
	}
	// Keep only the first path segment (ignore sub-paths if any)
	title = strings.SplitN(after, "/", 2)[0]
	return lang, title, nil
}
