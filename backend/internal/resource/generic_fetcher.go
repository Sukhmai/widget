package resource

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"
	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// GenericFetcher implements Fetcher for any URL without a dedicated source —
// the fallback when DetectSource returns SourceGeneric.
//
// It scrapes the page's OpenGraph / standard meta tags best-effort for a
// thumbnail, title, description and site name, and — only when the page declares
// og:type == "article" — extracts the full readable body via readability. Every
// field degrades to empty rather than erroring: a blocked or empty fetch still
// yields a card from the hostname + a favicon guess, so the resource is always
// marked fetched instead of dead-ending in a perpetual loading state.
type GenericFetcher struct {
	httpClient *http.Client
}

func NewGenericFetcher() *GenericFetcher {
	return &GenericFetcher{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

var (
	reMetaTag     = regexp.MustCompile(`(?is)<meta\b[^>]*?>`)
	reLinkTag     = regexp.MustCompile(`(?is)<link\b[^>]*?>`)
	reTitleTag    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reAttrProp    = regexp.MustCompile(`(?is)\bproperty\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	reAttrName    = regexp.MustCompile(`(?is)\bname\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	reAttrContent = regexp.MustCompile(`(?is)\bcontent\s*=\s*("[^"]*"|'[^']*')`)
	reAttrRel     = regexp.MustCompile(`(?is)\brel\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	reAttrHref    = regexp.MustCompile(`(?is)\bhref\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
)

// Fetch implements Fetcher. It never errors — see the type doc.
func (f *GenericFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	body, finalURL := f.fetchHTML(ctx, resourceURL)
	base, _ := url.Parse(finalURL)

	meta := parseMetaTags(body)

	title := firstNonEmpty(meta["og:title"], meta["twitter:title"], extractTitle(body))
	description := firstNonEmpty(meta["og:description"], meta["twitter:description"], meta["description"])

	image := firstNonEmpty(meta["og:image"], meta["twitter:image"])
	if image != "" {
		image = resolveURL(base, image)
	}

	siteName := meta["og:site_name"]
	if siteName == "" && base != nil {
		siteName = prettyHostName(base.Hostname())
	}

	isArticle := strings.EqualFold(meta["og:type"], "article")
	author := firstNonEmpty(meta["article:author"], meta["author"])

	var bodyText string
	if isArticle && len(body) > 0 {
		if art, err := readability.FromReader(bytes.NewReader(body), base); err == nil {
			bodyText = strings.TrimSpace(art.TextContent)
			if title == "" {
				title = art.Title
			}
			if siteName == "" {
				siteName = art.SiteName
			}
		}
	}

	out := &resourcev1.GenericMetadata{
		Url:         firstNonEmpty(meta["og:url"], finalURL),
		Title:       title,
		Description: description,
		ImageUrl:    image,
		FaviconUrl:  extractFavicon(body, base),
		SiteName:    siteName,
		Author:      author,
		IsArticle:   isArticle,
		BodyText:    bodyText,
	}

	return protojson.Marshal(out)
}

// fetchHTML issues a single best-effort GET. It always returns the final URL
// (after following redirects), and the body only when the response was a
// readable 200; a non-200 or transport error yields a nil body so the caller
// still produces a minimal card from the URL alone.
func (f *GenericFetcher) fetchHTML(ctx context.Context, rawURL string) (body []byte, finalURL string) {
	finalURL = rawURL

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, finalURL
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, finalURL
	}
	defer resp.Body.Close()

	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if resp.StatusCode != http.StatusOK {
		return nil, finalURL
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, finalURL
	}
	return b, finalURL
}

// parseMetaTags collects <meta> property/name → content pairs, keyed lowercase.
// The first occurrence of a key wins. Content values are HTML-unescaped.
func parseMetaTags(htmlBytes []byte) map[string]string {
	out := make(map[string]string)
	for _, tag := range reMetaTag.FindAll(htmlBytes, -1) {
		key := attrValue(reAttrProp, tag)
		if key == "" {
			key = attrValue(reAttrName, tag)
		}
		if key == "" {
			continue
		}
		content := attrValue(reAttrContent, tag)
		if content == "" {
			continue
		}
		key = strings.ToLower(key)
		if _, ok := out[key]; !ok {
			out[key] = html.UnescapeString(content)
		}
	}
	return out
}

// extractTitle pulls the <title> text as a fallback when og:title is absent.
func extractTitle(htmlBytes []byte) string {
	m := reTitleTag.FindSubmatch(htmlBytes)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(string(m[1])))
}

// extractFavicon returns the first <link rel="...icon..."> href resolved to an
// absolute URL, falling back to /favicon.ico on the page's host.
func extractFavicon(htmlBytes []byte, base *url.URL) string {
	for _, tag := range reLinkTag.FindAll(htmlBytes, -1) {
		if !strings.Contains(strings.ToLower(attrValue(reAttrRel, tag)), "icon") {
			continue
		}
		if href := attrValue(reAttrHref, tag); href != "" {
			return resolveURL(base, href)
		}
	}
	if base != nil && base.Host != "" {
		return base.Scheme + "://" + base.Host + "/favicon.ico"
	}
	return ""
}

// attrValue extracts an HTML attribute value via the given precompiled regex,
// stripping surrounding quotes.
func attrValue(re *regexp.Regexp, tag []byte) string {
	m := re.FindSubmatch(tag)
	if m == nil {
		return ""
	}
	return strings.Trim(strings.TrimSpace(string(m[1])), `"'`)
}

// resolveURL turns a possibly-relative ref into an absolute URL against base.
func resolveURL(base *url.URL, ref string) string {
	u, err := url.Parse(ref)
	if err != nil || base == nil {
		return ref
	}
	return base.ResolveReference(u).String()
}

// prettyHostName turns a bare hostname into a friendlier site label, used only
// when the page exposes no og:site_name — e.g. "www.jacobin.com" → "Jacobin".
// It strips a leading "www." and the TLD, then capitalises. Multi-part TLDs
// (co.uk) aren't special-cased; this is best-effort.
func prettyHostName(host string) string {
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	name := host
	if labels := strings.Split(host, "."); len(labels) >= 2 {
		name = labels[len(labels)-2] // the label before the TLD
	}
	if name == "" {
		return host
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
