package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// BlueskyFetcher implements Fetcher for bsky.app post URLs.
// It uses the public oEmbed endpoint — no API key required.
type BlueskyFetcher struct {
	httpClient *http.Client
}

func NewBlueskyFetcher() *BlueskyFetcher {
	return &BlueskyFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch implements Fetcher.
func (f *BlueskyFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	oembedURL := "https://embed.bsky.app/oembed?omit_script=true&url=" + url.QueryEscape(resourceURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("bluesky: build oembed request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bluesky: oembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("bluesky: oembed returned %d: %s", resp.StatusCode, b)
	}

	var oe struct {
		AuthorName string `json:"author_name"`
		AuthorURL  string `json:"author_url"`
		HTML       string `json:"html"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&oe); err != nil {
		return nil, fmt.Errorf("bluesky: decode oembed response: %w", err)
	}

	handle := extractHandle(oe.AuthorURL)
	text := extractTweetText(oe.HTML)

	meta := &resourcev1.BlueskyPostMetadata{
		PostUrl:    resourceURL,
		Text:       text,
		AuthorName: oe.AuthorName,
		Handle:     handle,
	}
	return protojson.Marshal(meta)
}
