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

// TikTokFetcher implements Fetcher for tiktok.com and vm.tiktok.com URLs.
// It uses the public oEmbed endpoint — no API key required.
type TikTokFetcher struct {
	httpClient *http.Client
}

func NewTikTokFetcher() *TikTokFetcher {
	return &TikTokFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch implements Fetcher.
func (f *TikTokFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	oembedURL := "https://www.tiktok.com/oembed?url=" + url.QueryEscape(resourceURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("tiktok: build oembed request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bot)")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiktok: oembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("tiktok: oembed returned %d: %s", resp.StatusCode, b)
	}

	var oe struct {
		Title        string `json:"title"`
		AuthorURL    string `json:"author_url"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&oe); err != nil {
		return nil, fmt.Errorf("tiktok: decode oembed response: %w", err)
	}

	embedURL := ""
	if vid := extractTikTokVideoID(resourceURL); vid != "" {
		embedURL = "https://www.tiktok.com/player/v1/" + vid + "?music_info=0&description=0&native_context_menu=0&rel=0"
	}

	meta := &resourcev1.TikTokVideoMetadata{
		CoverUrl:  oe.ThumbnailURL,
		Title:     oe.Title,
		Creator:   extractTikTokHandle(oe.AuthorURL),
		TiktokUrl: resourceURL,
		EmbedUrl:  embedURL,
	}
	return protojson.Marshal(meta)
}

// extractTikTokVideoID extracts the numeric video ID from a canonical TikTok
// video URL (tiktok.com/@user/video/{id}). Returns "" for short links or
// unrecognised patterns — the caller skips embed_url in that case.
func extractTikTokVideoID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Expect: ["@user", "video", "{id}"]
	if len(parts) >= 3 && parts[1] == "video" {
		return parts[2]
	}
	return ""
}

// extractTikTokHandle pulls the @handle from a TikTok profile URL like
// https://www.tiktok.com/@charlidamelio. Returns the raw author_url if the
// expected @-segment is not found.
func extractTikTokHandle(authorURL string) string {
	u, err := url.Parse(authorURL)
	if err != nil {
		return authorURL
	}
	// Path is "/@username" — grab the segment that starts with @.
	for _, seg := range strings.Split(u.Path, "/") {
		if strings.HasPrefix(seg, "@") {
			return seg
		}
	}
	return authorURL
}
