package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// SteamFetcher implements Fetcher for Steam store URLs (store.steampowered.com
// and s.team short links). Metadata comes from the Steam Store API — no API
// key required.
type SteamFetcher struct {
	httpClient *http.Client
}

func NewSteamFetcher() *SteamFetcher {
	return &SteamFetcher{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

var reSteamAppID = regexp.MustCompile(`/app/(\d+)`)

// Fetch implements Fetcher.
func (f *SteamFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	storeURL, err := f.resolveStoreURL(ctx, resourceURL)
	if err != nil {
		return nil, err
	}

	m := reSteamAppID.FindStringSubmatch(storeURL)
	if m == nil {
		return nil, fmt.Errorf("steam: cannot extract app ID from %q", storeURL)
	}
	appID := m[1]

	data, err := f.fetchAppDetails(ctx, appID)
	if err != nil {
		return nil, err
	}

	var releaseYear int32
	if data.ReleaseDate.Date != "" {
		releaseYear = parseSteamYear(data.ReleaseDate.Date)
	}

	var developer string
	if len(data.Developers) > 0 {
		developer = data.Developers[0]
	}

	var publisher string
	if len(data.Publishers) > 0 {
		publisher = data.Publishers[0]
	}

	meta := &resourcev1.SteamGameMetadata{
		CoverUrl:    data.HeaderImage,
		Title:       data.Name,
		Developer:   developer,
		Publisher:   publisher,
		ReleaseYear: releaseYear,
		SteamUrl:    canonicalSteamURL(appID, data.Name),
		AppId:       appID,
	}

	return protojson.Marshal(meta)
}

// resolveStoreURL follows s.team short links to their final store.steampowered.com
// destination. Returns the URL unchanged for direct store URLs.
func (f *SteamFetcher) resolveStoreURL(ctx context.Context, rawURL string) (string, error) {
	if strings.Contains(rawURL, "store.steampowered.com") {
		return rawURL, nil
	}
	// s.team short link — follow redirect
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("steam: build redirect request: %w", err)
	}
	// Don't follow automatically so we can inspect the Location header.
	noFollow := *f.httpClient
	noFollow.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := noFollow.Do(req)
	if err != nil {
		return "", fmt.Errorf("steam: resolve short link: %w", err)
	}
	resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("steam: no Location header from %q", rawURL)
	}
	return loc, nil
}

type steamAppData struct {
	Name        string   `json:"name"`
	HeaderImage string   `json:"header_image"`
	Developers  []string `json:"developers"`
	Publishers  []string `json:"publishers"`
	ReleaseDate struct {
		Date string `json:"date"`
	} `json:"release_date"`
}

func (f *SteamFetcher) fetchAppDetails(ctx context.Context, appID string) (*steamAppData, error) {
	apiURL := "https://store.steampowered.com/api/appdetails?appids=" + appID + "&cc=us&l=en"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("steam: build api request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steam: api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("steam: api returned %d: %s", resp.StatusCode, b)
	}

	// Response shape: { "<appid>": { "success": bool, "data": { ... } } }
	var raw map[string]struct {
		Success bool         `json:"success"`
		Data    steamAppData `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("steam: decode api response: %w", err)
	}

	entry, ok := raw[appID]
	if !ok || !entry.Success {
		return nil, fmt.Errorf("steam: api returned no data for app %q", appID)
	}

	return &entry.Data, nil
}

// parseSteamYear extracts the year from Steam's date string (e.g. "21 Aug, 2013"
// or "Q4 2023"). Returns 0 on parse failure.
func parseSteamYear(date string) int32 {
	parts := strings.Fields(date)
	for i := len(parts) - 1; i >= 0; i-- {
		y, err := strconv.Atoi(parts[i])
		if err == nil && y > 1970 && y < 2100 {
			return int32(y)
		}
	}
	return 0
}

// canonicalSteamURL builds a clean store URL from the app ID and name.
func canonicalSteamURL(appID, name string) string {
	slug := strings.ToLower(name)
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "_")
	slug = strings.Trim(slug, "_")
	if slug == "" {
		return "https://store.steampowered.com/app/" + appID + "/"
	}
	return "https://store.steampowered.com/app/" + appID + "/" + slug + "/"
}
