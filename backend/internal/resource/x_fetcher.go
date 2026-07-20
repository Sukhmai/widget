package resource

import (
	"context"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// XFetcher implements Fetcher for twitter.com / x.com tweet URLs.
// It uses the public oEmbed endpoint — no API key required.
type XFetcher struct {
	httpClient *http.Client
}

func NewXFetcher() *XFetcher {
	return &XFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

var (
	reTweetPara   = regexp.MustCompile(`(?i)<p[^>]*>(.*?)</p>`)
	reAnchorHref  = regexp.MustCompile(`(?i)<a\s[^>]*href="([^"]*)"[^>]*>[\s\S]*?</a>`)
	reHTMLTag     = regexp.MustCompile(`<[^>]+>`)
)

// Fetch implements Fetcher.
func (f *XFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	oembedURL := "https://publish.x.com/oembed?omit_script=true&url=" + url.QueryEscape(resourceURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("x: build oembed request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("x: oembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("x: oembed returned %d: %s", resp.StatusCode, b)
	}

	var oe struct {
		AuthorName string `json:"author_name"`
		AuthorURL  string `json:"author_url"`
		HTML       string `json:"html"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&oe); err != nil {
		return nil, fmt.Errorf("x: decode oembed response: %w", err)
	}

	handle := extractHandle(oe.AuthorURL)
	text := extractTweetText(oe.HTML)

	meta := &resourcev1.TweetMetadata{
		TweetUrl:   resourceURL,
		Text:       text,
		AuthorName: oe.AuthorName,
		Handle:     handle,
	}
	return protojson.Marshal(meta)
}

// extractHandle pulls the username from the author_url path, e.g.
// "https://twitter.com/jack" → "jack".
func extractHandle(authorURL string) string {
	u, err := url.Parse(authorURL)
	if err != nil {
		return ""
	}
	return strings.Trim(u.Path, "/")
}

// extractTweetText pulls the tweet body out of the oEmbed HTML blob.
// <a> tags are replaced with their href so URLs stay in the text as plain
// strings; the frontend renders them as clickable links.
func extractTweetText(html string) string {
	m := reTweetPara.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	text := reAnchorHref.ReplaceAllString(m[1], "$1")
	text = reHTMLTag.ReplaceAllString(text, "")
	text = htmlpkg.UnescapeString(text)
	return strings.TrimSpace(text)
}
