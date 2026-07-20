package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// RedditFetcher implements Fetcher for reddit.com post URLs and redd.it short links.
// Uses the public Reddit JSON API keyed only on the post ID, so slugs, query
// strings, and subdomains (old., www.) are all irrelevant.
type RedditFetcher struct {
	httpClient *http.Client
}

func NewRedditFetcher() *RedditFetcher {
	return &RedditFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// sentinelThumbnails is the set of non-image sentinel strings Reddit uses.
var sentinelThumbnails = map[string]bool{
	"self": true, "default": true, "nsfw": true, "spoiler": true, "": true,
}

// rePostID matches the base-36 post ID in reddit.com and redd.it URLs.
//
//	reddit.com/r/sub/comments/{id}/slug  →  group 1
//	redd.it/{id}                          →  group 2
var rePostID = regexp.MustCompile(`(?:/comments/|redd\.it/)([a-z0-9]+)`)

// Fetch implements Fetcher.
func (f *RedditFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	m := rePostID.FindStringSubmatch(resourceURL)
	if m == nil {
		return nil, fmt.Errorf("reddit: cannot extract post ID from %q", resourceURL)
	}
	postID := m[1]

	// Minimal URL form — Reddit ignores the slug, only the ID matters.
	apiURL := "https://www.reddit.com/comments/" + postID + ".json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("reddit: build request: %w", err)
	}
	// Reddit's CDN inspects the UA and Accept headers; browser values are
	// required to avoid 403s from the CDN layer.
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reddit: fetch json: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("reddit: api returned %d: %s", resp.StatusCode, b)
	}

	// Response is [post listing, comments listing].
	// Post data is at [0].data.children[0].data.
	var listings []struct {
		Data struct {
			Children []struct {
				Data redditPost `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listings); err != nil {
		return nil, fmt.Errorf("reddit: decode response: %w", err)
	}
	if len(listings) == 0 || len(listings[0].Data.Children) == 0 {
		return nil, fmt.Errorf("reddit: empty listing for post %q", postID)
	}

	post := listings[0].Data.Children[0].Data
	meta := &resourcev1.RedditPostMetadata{
		Title:     post.Title,
		Author:    post.Author,
		Subreddit: post.Subreddit,
		BodyText:  post.Selftext,
		CoverUrl:  post.coverImage(),
		RedditUrl: "https://www.reddit.com" + post.Permalink,
	}
	return protojson.Marshal(meta)
}

type redditPost struct {
	Title     string `json:"title"`
	Author    string `json:"author"`
	Subreddit string `json:"subreddit"`
	Selftext  string `json:"selftext"`
	Permalink string `json:"permalink"`
	Thumbnail string `json:"thumbnail"`
	Preview   struct {
		Images []struct {
			Source struct {
				URL string `json:"url"`
			} `json:"source"`
		} `json:"images"`
	} `json:"preview"`
}

// coverImage returns the best available image URL.
// Reddit HTML-encodes preview image URLs, so we decode them.
func (p *redditPost) coverImage() string {
	if len(p.Preview.Images) > 0 {
		if u := html.UnescapeString(p.Preview.Images[0].Source.URL); u != "" {
			return u
		}
	}
	if !sentinelThumbnails[p.Thumbnail] {
		return p.Thumbnail
	}
	return ""
}
