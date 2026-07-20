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

// InstagramFetcher implements Fetcher for instagram.com and instagr.am URLs.
// It uses noembed.com as a token-free oEmbed proxy — no Meta app credentials needed.
type InstagramFetcher struct {
	httpClient *http.Client
}

func NewInstagramFetcher() *InstagramFetcher {
	return &InstagramFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch implements Fetcher.
func (f *InstagramFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	oembedURL := "https://noembed.com/embed?url=" + url.QueryEscape(resourceURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("instagram: build noembed request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram: noembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("instagram: noembed returned %d: %s", resp.StatusCode, b)
	}

	var oe struct {
		AuthorName   string `json:"author_name"`
		AuthorURL    string `json:"author_url"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&oe); err != nil {
		return nil, fmt.Errorf("instagram: decode noembed response: %w", err)
	}

	handle := extractHandle(oe.AuthorURL)
	isReel := strings.Contains(resourceURL, "/reel/")

	embedURL := ""
	if isReel {
		if sc := extractInstagramShortcode(resourceURL); sc != "" {
			embedURL = "https://www.instagram.com/reel/" + sc + "/embed/"
		}
	}

	meta := &resourcev1.InstagramPostMetadata{
		PostUrl:    resourceURL,
		CoverUrl:   oe.ThumbnailURL,
		Handle:     handle,
		AuthorName: oe.AuthorName,
		IsReel:     isReel,
		EmbedUrl:   embedURL,
	}
	return protojson.Marshal(meta)
}

// extractInstagramShortcode extracts the shortcode from a canonical Instagram
// post or reel URL (instagram.com/p/{shortcode}/ or instagram.com/reel/{shortcode}/).
func extractInstagramShortcode(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Expect: ["p", "{shortcode}"] or ["reel", "{shortcode}"]
	if len(parts) >= 2 && (parts[0] == "p" || parts[0] == "reel") {
		return parts[1]
	}
	return ""
}
