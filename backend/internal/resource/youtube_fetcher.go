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

// YouTubeFetcher implements Fetcher for youtube.com and youtu.be URLs.
// It uses the public oEmbed endpoint — no API key required.
type YouTubeFetcher struct {
	httpClient *http.Client
}

func NewYouTubeFetcher() *YouTubeFetcher {
	return &YouTubeFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch implements Fetcher.
func (f *YouTubeFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	videoID, err := extractYouTubeVideoID(resourceURL)
	if err != nil {
		return nil, err
	}

	oembedURL := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(resourceURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("youtube: build oembed request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube: oembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("youtube: oembed returned %d: %s", resp.StatusCode, b)
	}

	var oe struct {
		Title        string `json:"title"`
		AuthorName   string `json:"author_name"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&oe); err != nil {
		return nil, fmt.Errorf("youtube: decode oembed response: %w", err)
	}

	meta := &resourcev1.YouTubeVideoMetadata{
		VideoId:     videoID,
		Title:       oe.Title,
		ChannelName: oe.AuthorName,
		CoverUrl:    oe.ThumbnailURL,
		EmbedUrl:    "https://www.youtube.com/embed/" + videoID,
	}
	return protojson.Marshal(meta)
}

// extractYouTubeVideoID parses the video ID from a youtube.com/watch?v= or
// youtu.be/ URL.
func extractYouTubeVideoID(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("youtube: invalid url %q: %w", rawURL, err)
	}
	// youtu.be/<id>
	if strings.Contains(u.Host, "youtu.be") {
		id := strings.Trim(u.Path, "/")
		if id == "" {
			return "", fmt.Errorf("youtube: no video id in %q", rawURL)
		}
		return id, nil
	}
	// youtube.com/watch?v=<id>  or  youtube.com/shorts/<id>
	if v := u.Query().Get("v"); v != "" {
		return v, nil
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "shorts" {
		return parts[1], nil
	}
	return "", fmt.Errorf("youtube: cannot extract video id from %q", rawURL)
}
