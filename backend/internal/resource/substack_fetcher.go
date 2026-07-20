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

// SubstackFetcher implements Fetcher for *.substack.com/p/ URLs.
// It calls the public Substack API (no key required).
type SubstackFetcher struct {
	httpClient *http.Client
}

func NewSubstackFetcher() *SubstackFetcher {
	return &SubstackFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch implements Fetcher.
func (f *SubstackFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	subdomain, slug, err := parseSubstackURL(resourceURL)
	if err != nil {
		return nil, err
	}

	apiURL := "https://" + subdomain + ".substack.com/api/v1/posts/" + slug

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("substack: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("substack: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("substack: API returned %d: %s", resp.StatusCode, b)
	}

	var data struct {
		Title            string  `json:"title"`
		Subtitle         string  `json:"subtitle"`
		BodyHTML         string  `json:"body_html"`
		CoverImage       *string `json:"cover_image"`
		CanonicalURL     string  `json:"canonical_url"`
		PublishedBylines []struct {
			Name string `json:"name"`
		} `json:"publishedBylines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("substack: decode response: %w", err)
	}

	var coverURL string
	if data.CoverImage != nil {
		coverURL = *data.CoverImage
	}
	var authorName string
	if len(data.PublishedBylines) > 0 {
		authorName = data.PublishedBylines[0].Name
	}
	canonicalURL := data.CanonicalURL
	if canonicalURL == "" {
		canonicalURL = resourceURL
	}

	meta := &resourcev1.SubstackPostMetadata{
		Title:           data.Title,
		AuthorName:      authorName,
		PublicationName: substackPublicationName(subdomain),
		CoverUrl:        coverURL,
		CanonicalUrl:    canonicalURL,
		Subtitle:        data.Subtitle,
		BodyText:        stripHTML(data.BodyHTML),
	}
	return protojson.Marshal(meta)
}

// parseSubstackURL extracts the subdomain and post slug from:
//   - https://{subdomain}.substack.com/p/{slug}
//   - https://open.substack.com/pub/{subdomain}/p/{slug}
func parseSubstackURL(rawURL string) (subdomain, slug string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("substack: parse URL: %w", err)
	}

	// open.substack.com/pub/{publication}/p/{slug}
	if u.Hostname() == "open.substack.com" {
		after, ok := strings.CutPrefix(u.Path, "/pub/")
		if !ok || after == "" {
			return "", "", fmt.Errorf("substack: no publication in path %q", u.Path)
		}
		pubAndRest := strings.SplitN(after, "/p/", 2)
		if len(pubAndRest) != 2 || pubAndRest[1] == "" {
			return "", "", fmt.Errorf("substack: no post slug in path %q", u.Path)
		}
		subdomain = strings.SplitN(pubAndRest[0], "/", 2)[0]
		slug = strings.SplitN(pubAndRest[1], "/", 2)[0]
		return subdomain, slug, nil
	}

	// {subdomain}.substack.com/p/{slug}
	parts := strings.SplitN(u.Hostname(), ".", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("substack: unexpected host %q", u.Hostname())
	}
	subdomain = parts[0]

	after, ok := strings.CutPrefix(u.Path, "/p/")
	if !ok || after == "" {
		return "", "", fmt.Errorf("substack: no post slug in path %q", u.Path)
	}
	slug = strings.SplitN(after, "/", 2)[0]
	return subdomain, slug, nil
}

// substackPublicationName converts a subdomain like "lenny-rachitsky" to
// "Lenny Rachitsky".
func substackPublicationName(subdomain string) string {
	words := strings.Split(subdomain, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// stripHTML removes HTML tags and decodes common entities, producing plain text
// suitable for storage. Block-level tags (p, div, li, br) become newlines so
// paragraph breaks are preserved.
func stripHTML(html string) string {
	var b strings.Builder
	inTag := false
	pendingNewline := false

	flushNewline := func() {
		if pendingNewline {
			b.WriteByte('\n')
			pendingNewline = false
		}
	}

	i := 0
	for i < len(html) {
		c := html[i]
		switch {
		case c == '<':
			inTag = true
			// Peek at tag name to emit a newline for block elements.
			j := i + 1
			if j < len(html) && html[j] == '/' {
				j++
			}
			end := j
			for end < len(html) && html[end] != '>' && html[end] != ' ' {
				end++
			}
			tag := strings.ToLower(html[j:end])
			switch tag {
			case "p", "div", "li", "br", "h1", "h2", "h3", "h4", "h5", "h6":
				pendingNewline = true
			}
			i++
		case c == '>':
			inTag = false
			i++
		case inTag:
			i++
		case c == '&':
			flushNewline()
			end := i + 1
			for end < len(html) && html[end] != ';' && end-i < 10 {
				end++
			}
			entity := html[i : end+1]
			switch entity {
			case "&amp;":
				b.WriteByte('&')
			case "&lt;":
				b.WriteByte('<')
			case "&gt;":
				b.WriteByte('>')
			case "&quot;":
				b.WriteByte('"')
			case "&#39;", "&apos;":
				b.WriteByte('\'')
			case "&nbsp;":
				b.WriteByte(' ')
			default:
				b.WriteString(entity)
			}
			i = end + 1
		default:
			flushNewline()
			b.WriteByte(c)
			i++
		}
	}

	// Collapse runs of whitespace/newlines.
	raw := b.String()
	var out strings.Builder
	prevNL := false
	for _, r := range raw {
		if r == '\n' {
			if !prevNL {
				out.WriteRune('\n')
			}
			prevNL = true
		} else {
			prevNL = false
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}
