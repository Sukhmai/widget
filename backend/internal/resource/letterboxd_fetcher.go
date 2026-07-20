package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// LetterboxdFetcher implements Fetcher for letterboxd.com and boxd.it URLs.
//
// TMDB is the resilient backbone: it supplies poster, title, year, director,
// genres, runtime and overview. The Letterboxd page itself sits behind
// Cloudflare and is fetched best-effort only for the community rating (which
// TMDB lacks) and the embedded TMDB id; if that fetch is blocked the card still
// renders fully from TMDB, just without the rating.
type LetterboxdFetcher struct {
	tmdbToken  string // TMDB v4 read access token, sent as a Bearer header
	httpClient *http.Client
}

func NewLetterboxdFetcher(tmdbToken string) *LetterboxdFetcher {
	return &LetterboxdFetcher{
		tmdbToken:  tmdbToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// browserUA (see useragents.go) is used here because Letterboxd's Cloudflare
// layer rejects obvious bots; we present as a browser for the best-effort scrape.

const tmdbImageBase = "https://image.tmdb.org/t/p/w500"

var (
	reLbxdTmdbData = regexp.MustCompile(`data-tmdb-id="(\d+)"`)
	reLbxdTmdbLink = regexp.MustCompile(`themoviedb\.org/movie/(\d+)`)
	reLbxdRating   = regexp.MustCompile(`"ratingValue":\s*([0-9.]+)`)
)

// Fetch implements Fetcher.
func (f *LetterboxdFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	// Best-effort scrape: resolves boxd.it short links via redirect, and pulls
	// the TMDB id + community rating from the page body when not blocked.
	canonicalURL, tmdbID, rating := f.scrapeLetterboxd(ctx, resourceURL)

	// Fall back to a TMDB title/year search if the page didn't yield an id
	// (e.g. Cloudflare returned 403 with no parseable body).
	if tmdbID == "" {
		title, year := parseFilmSlug(canonicalURL)
		if title != "" {
			tmdbID = f.searchTMDB(ctx, title, year)
		}
	}
	if tmdbID == "" {
		return nil, fmt.Errorf("letterboxd: could not resolve TMDB id for %q", resourceURL)
	}

	movie, err := f.fetchTMDB(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	var posterURL string
	if movie.PosterPath != "" {
		posterURL = tmdbImageBase + movie.PosterPath
	}

	var director string
	for _, c := range movie.Credits.Crew {
		if c.Job == "Director" {
			director = c.Name
			break
		}
	}

	genres := make([]string, 0, len(movie.Genres))
	for _, g := range movie.Genres {
		genres = append(genres, g.Name)
	}

	meta := &resourcev1.MovieMetadata{
		PosterUrl:        posterURL,
		Title:            movie.Title,
		ReleaseYear:      yearFromDate(movie.ReleaseDate),
		Director:         director,
		Genres:           genres,
		RuntimeMinutes:   int32(movie.Runtime),
		Overview:         movie.Overview,
		LetterboxdRating: rating,
		LetterboxdUrl:    canonicalLetterboxdURL(canonicalURL),
		TmdbId:           tmdbID,
	}

	return protojson.Marshal(meta)
}

// scrapeLetterboxd performs the single best-effort page GET. It always returns
// the final URL (after following the boxd.it redirect, even on a non-200), and
// returns the TMDB id + rating only when the body was readable.
func (f *LetterboxdFetcher) scrapeLetterboxd(ctx context.Context, rawURL string) (finalURL, tmdbID string, rating float64) {
	finalURL = rawURL

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return finalURL, "", 0
	}
	req.Header.Set("User-Agent", browserUA)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return finalURL, "", 0
	}
	defer resp.Body.Close()

	// resp.Request reflects the final hop after redirects (resolves boxd.it).
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if resp.StatusCode != http.StatusOK {
		return finalURL, "", 0 // blocked or moved; we still have the canonical URL
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return finalURL, "", 0
	}

	if m := reLbxdTmdbData.FindSubmatch(body); m != nil {
		tmdbID = string(m[1])
	} else if m := reLbxdTmdbLink.FindSubmatch(body); m != nil {
		tmdbID = string(m[1])
	}

	if m := reLbxdRating.FindSubmatch(body); m != nil {
		if r, err := strconv.ParseFloat(string(m[1]), 64); err == nil {
			rating = r
		}
	}

	return finalURL, tmdbID, rating
}

// tmdbMovie holds only the fields we need from the TMDB movie details endpoint
// (with append_to_response=credits).
type tmdbMovie struct {
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
	Runtime     int    `json:"runtime"`
	Overview    string `json:"overview"`
	PosterPath  string `json:"poster_path"`
	Genres      []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Credits struct {
		Crew []struct {
			Job  string `json:"job"`
			Name string `json:"name"`
		} `json:"crew"`
	} `json:"credits"`
}

func (f *LetterboxdFetcher) fetchTMDB(ctx context.Context, id string) (*tmdbMovie, error) {
	apiURL := "https://api.themoviedb.org/3/movie/" + id + "?append_to_response=credits"

	resp, err := f.tmdbGet(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("letterboxd: tmdb request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("letterboxd: tmdb request returned %d: %s", resp.StatusCode, b)
	}

	var movie tmdbMovie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("letterboxd: decode tmdb response: %w", err)
	}
	return &movie, nil
}

// searchTMDB is the fallback path when the Letterboxd page didn't expose a
// TMDB id. Returns empty string if no match is found.
func (f *LetterboxdFetcher) searchTMDB(ctx context.Context, title, year string) string {
	q := url.Values{}
	q.Set("query", title)
	if year != "" {
		q.Set("year", year)
	}
	apiURL := "https://api.themoviedb.org/3/search/movie?" + q.Encode()

	resp, err := f.tmdbGet(ctx, apiURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var result struct {
		Results []struct {
			ID int64 `json:"id"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	if len(result.Results) == 0 {
		return ""
	}
	return strconv.FormatInt(result.Results[0].ID, 10)
}

// tmdbGet issues a GET to the TMDB API authenticated with the v4 read access
// token as a Bearer header.
func (f *LetterboxdFetcher) tmdbGet(ctx context.Context, apiURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+f.tmdbToken)
	req.Header.Set("Accept", "application/json")
	return f.httpClient.Do(req)
}

// parseFilmSlug extracts a searchable title and year from a Letterboxd film URL.
// e.g. https://letterboxd.com/film/backrooms-2026/ → ("backrooms", "2026")
func parseFilmSlug(rawURL string) (title, year string) {
	_, slug, found := strings.Cut(rawURL, "/film/")
	if !found {
		return "", ""
	}
	slug, _, _ = strings.Cut(slug, "/")
	if slug == "" {
		return "", ""
	}

	parts := strings.Split(slug, "-")
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if len(last) == 4 && isAllDigits(last) {
			year = last
			parts = parts[:len(parts)-1]
		}
	}
	return strings.Join(parts, " "), year
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

// yearFromDate parses the leading year from a TMDB release_date ("2026-03-15").
func yearFromDate(d string) int32 {
	if len(d) >= 4 {
		if y, err := strconv.Atoi(d[:4]); err == nil {
			return int32(y)
		}
	}
	return 0
}

// canonicalLetterboxdURL returns the URL only if it points at letterboxd.com,
// so we never persist an unresolved boxd.it short link or a TMDB search miss.
func canonicalLetterboxdURL(u string) string {
	if strings.Contains(u, "letterboxd.com") {
		return u
	}
	return ""
}
