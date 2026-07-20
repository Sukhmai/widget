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

// IMDBFetcher implements Fetcher for imdb.com title URLs.
// TMDB is the backbone: metadata is resolved via TMDB's /find endpoint using
// the IMDB tt-ID extracted from the URL. If /find returns no results the
// fetcher falls back to scraping the IMDB page title and running a TMDB
// title/year search — same ladder the LetterboxdFetcher uses.
type IMDBFetcher struct {
	tmdbToken  string
	httpClient *http.Client
}

func NewIMDBFetcher(tmdbToken string) *IMDBFetcher {
	return &IMDBFetcher{
		tmdbToken:  tmdbToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

var reIMDBID = regexp.MustCompile(`/title/(tt\d+)`)

// Fetch implements Fetcher.
func (f *IMDBFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	imdbID := extractIMDBID(resourceURL)

	// Primary: resolve via TMDB /find using the tt-ID.
	var tmdbID string
	if imdbID != "" {
		tmdbID = f.findByIMDBID(ctx, imdbID)
	}

	// Fallback: scrape the IMDB page for title + year, then TMDB title search.
	if tmdbID == "" {
		title, year := f.scrapeIMDBTitle(ctx, resourceURL)
		if title != "" {
			tmdbID = f.searchTMDB(ctx, title, year)
		}
	}

	if tmdbID == "" {
		return nil, fmt.Errorf("imdb: could not resolve TMDB id for %q", resourceURL)
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
		PosterUrl:      posterURL,
		Title:          movie.Title,
		ReleaseYear:    yearFromDate(movie.ReleaseDate),
		Director:       director,
		Genres:         genres,
		RuntimeMinutes: int32(movie.Runtime),
		Overview:       movie.Overview,
		ImdbUrl:        canonicalIMDBURL(resourceURL, imdbID),
		TmdbId:         tmdbID,
	}
	return protojson.Marshal(meta)
}

// extractIMDBID pulls the tt-ID out of an IMDB title URL.
func extractIMDBID(rawURL string) string {
	m := reIMDBID.FindStringSubmatch(rawURL)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// canonicalIMDBURL returns a clean imdb.com/title/tt.../  URL.
// Falls back to the original URL if the ID is unknown.
func canonicalIMDBURL(rawURL, imdbID string) string {
	if imdbID != "" {
		return "https://www.imdb.com/title/" + imdbID + "/"
	}
	return rawURL
}

// findByIMDBID calls TMDB /find to map an IMDB tt-ID to a TMDB movie ID.
// Returns empty string if the endpoint returns no movie results.
func (f *IMDBFetcher) findByIMDBID(ctx context.Context, imdbID string) string {
	apiURL := "https://api.themoviedb.org/3/find/" + imdbID + "?external_source=imdb_id"
	resp, err := f.tmdbGet(ctx, apiURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var result struct {
		MovieResults []struct {
			ID int64 `json:"id"`
		} `json:"movie_results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.MovieResults) == 0 {
		return ""
	}
	return strconv.FormatInt(result.MovieResults[0].ID, 10)
}

// scrapeIMDBTitle fetches the IMDB page best-effort and extracts a title and
// year from the HTML <title> tag (e.g. "Inception (2010) - IMDb").
var reIMDBPageTitle = regexp.MustCompile(`<title[^>]*>([^<]+)</title>`)
var reIMDBYear = regexp.MustCompile(`\((\d{4})\)`)

func (f *IMDBFetcher) scrapeIMDBTitle(ctx context.Context, rawURL string) (title, year string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", ""
	}

	m := reIMDBPageTitle.FindSubmatch(body)
	if len(m) < 2 {
		return "", ""
	}
	raw := strings.TrimSpace(string(m[1]))
	// Strip " - IMDb" suffix and any trailing cruft.
	raw, _, _ = strings.Cut(raw, " - IMDb")
	raw = strings.TrimSpace(raw)

	if ym := reIMDBYear.FindStringSubmatch(raw); len(ym) == 2 {
		year = ym[1]
		raw = strings.TrimSpace(reIMDBYear.ReplaceAllString(raw, ""))
	}
	return raw, year
}

// searchTMDB mirrors the Letterboxd fetcher's title/year search fallback.
func (f *IMDBFetcher) searchTMDB(ctx context.Context, title, year string) string {
	q := url.Values{}
	q.Set("query", title)
	if year != "" {
		q.Set("year", year)
	}
	resp, err := f.tmdbGet(ctx, "https://api.themoviedb.org/3/search/movie?"+q.Encode())
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Results) == 0 {
		return ""
	}
	return strconv.FormatInt(result.Results[0].ID, 10)
}

// fetchTMDB calls /3/movie/{id}?append_to_response=credits — same endpoint as
// the Letterboxd fetcher; reuses the shared tmdbMovie struct.
func (f *IMDBFetcher) fetchTMDB(ctx context.Context, id string) (*tmdbMovie, error) {
	apiURL := "https://api.themoviedb.org/3/movie/" + id + "?append_to_response=credits"
	resp, err := f.tmdbGet(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("imdb: tmdb request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("imdb: tmdb request returned %d: %s", resp.StatusCode, b)
	}

	var movie tmdbMovie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("imdb: decode tmdb response: %w", err)
	}
	return &movie, nil
}

func (f *IMDBFetcher) tmdbGet(ctx context.Context, apiURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+f.tmdbToken)
	req.Header.Set("Accept", "application/json")
	return f.httpClient.Do(req)
}
