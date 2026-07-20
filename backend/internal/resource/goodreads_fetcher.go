package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// GoodreadsFetcher implements Fetcher for goodreads.com book URLs.
//
// Strategy (mirrors LetterboxdFetcher):
//  1. Best-effort scrape the Goodreads page (browser UA) to extract the ISBN-13.
//  2. If we get an ISBN, query Open Library by ISBN — most precise lookup.
//  3. If Cloudflare blocks or no ISBN is found, fall back to a title search
//     derived from the URL slug. The card still renders fully from Open Library
//     either way.
type GoodreadsFetcher struct {
	httpClient *http.Client
}

func NewGoodreadsFetcher() *GoodreadsFetcher {
	return &GoodreadsFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

var (
	reGoodreadsBookID = regexp.MustCompile(`/book/show/(\d+)`)
	reGoodreadsSlug   = regexp.MustCompile(`/book/show/\d+-([^/?#]+)`)
	// ISBN-13 appears in Goodreads pages inside JSON-LD and schema.org microdata.
	reISBN13 = regexp.MustCompile(`(?:"isbn13"|"isbn")\s*:\s*"(\d{13})"`)
	reISBN13Micro = regexp.MustCompile(`itemprop="isbn"[^>]*>(\d{13})<`)
)

// Fetch implements Fetcher.
func (f *GoodreadsFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	if reGoodreadsBookID.FindStringSubmatch(resourceURL) == nil {
		return nil, fmt.Errorf("goodreads: cannot extract book ID from %q", resourceURL)
	}

	isbn := f.scrapeISBN(ctx, resourceURL)

	book, olID, err := f.fetchOpenLibrary(ctx, isbn, resourceURL)
	if err != nil {
		return nil, err
	}

	var coverURL string
	if book.CoverI != 0 {
		coverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", book.CoverI)
	}

	var author string
	if len(book.AuthorName) > 0 {
		author = book.AuthorName[0]
	}

	meta := &resourcev1.BookMetadata{
		CoverUrl:        coverURL,
		Title:           book.Title,
		Author:          author,
		PublicationYear: int32(book.FirstPublishYear),
		GoodreadsUrl:    canonicalGoodreadsURL(resourceURL),
		OpenLibraryId:   olID,
	}

	return protojson.Marshal(meta)
}

// scrapeISBN performs a best-effort GET of the Goodreads page to extract the
// ISBN-13. Returns empty string if Cloudflare blocks the request or no ISBN
// is found — the caller falls back to a title search.
func (f *GoodreadsFetcher) scrapeISBN(ctx context.Context, rawURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", browserUA)

	resp, err := f.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}

	if m := reISBN13.FindSubmatch(body); m != nil {
		return string(m[1])
	}
	if m := reISBN13Micro.FindSubmatch(body); m != nil {
		return string(m[1])
	}
	return ""
}

type olDoc struct {
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	CoverI           int64    `json:"cover_i"`
	Key              string   `json:"key"` // e.g. "/works/OL1234W"
}

// fetchOpenLibrary queries Open Library. When an ISBN is available it uses
// that for a precise lookup; otherwise it falls back to the title slug from
// the Goodreads URL.
func (f *GoodreadsFetcher) fetchOpenLibrary(ctx context.Context, isbn, resourceURL string) (*olDoc, string, error) {
	var query string
	if isbn != "" {
		query = "isbn=" + isbn
	} else {
		m := reGoodreadsSlug.FindStringSubmatch(resourceURL)
		if m == nil {
			return nil, "", fmt.Errorf("goodreads: cannot extract title slug from %q", resourceURL)
		}
		title := strings.ReplaceAll(m[1], "-", " ")
		query = "title=" + url.QueryEscape(title)
	}

	apiURL := "https://openlibrary.org/search.json?" + query +
		"&fields=title,author_name,first_publish_year,cover_i,key&limit=1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("goodreads: build ol request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("goodreads: open library request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("goodreads: open library returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Docs []olDoc `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("goodreads: decode open library response: %w", err)
	}
	if len(result.Docs) == 0 {
		return nil, "", fmt.Errorf("goodreads: no open library result (isbn=%q, url=%q)", isbn, resourceURL)
	}

	doc := &result.Docs[0]
	return doc, doc.Key, nil
}

// canonicalGoodreadsURL strips query params and fragment from the URL.
func canonicalGoodreadsURL(u string) string {
	for i, c := range u {
		if c == '?' || c == '#' {
			return u[:i]
		}
	}
	return u
}
