package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// SpotifyFetcher implements Fetcher for open.spotify.com URLs.
// It manages a client credentials token internally, refreshing it when expired.
type SpotifyFetcher struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client

	tokenMu  sync.Mutex
	token    string
	tokenExp time.Time
}

func NewSpotifyFetcher(clientID, clientSecret string) *SpotifyFetcher {
	return &SpotifyFetcher{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch implements Fetcher. Only track URLs are supported; other Spotify
// content types return an error until their metadata structs are defined.
func (f *SpotifyFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	kind, id, err := parseSpotifyURL(resourceURL)
	if err != nil {
		return nil, err
	}
	if kind != "track" {
		return nil, fmt.Errorf("spotify fetcher: unsupported type %q (only track is supported)", kind)
	}

	token, err := f.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	track, err := f.fetchTrack(ctx, token, id)
	if err != nil {
		return nil, err
	}

	// Pick the largest cover image Spotify returns.
	var coverURL string
	if len(track.Album.Images) > 0 {
		coverURL = track.Album.Images[0].URL
	}

	var artistName string
	if len(track.Artists) > 0 {
		artistName = track.Artists[0].Name
	}

	deezerTrackID := f.deezerTrackID(ctx, track.Name, artistName, int32(track.DurationMS))

	meta := &resourcev1.MusicTrackMetadata{
		EmbedUrl:      "https://open.spotify.com/embed/track/" + id,
		Title:         track.Name,
		Artist:        artistName,
		Album:         track.Album.Name,
		CoverUrl:      coverURL,
		DurationMs:    int32(track.DurationMS),
		DeezerTrackId: deezerTrackID,
	}

	b, err := protojson.Marshal(meta)
	return b, err
}

// parseSpotifyURL extracts the content type and ID from a Spotify URL.
// e.g. https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh → ("track", "4iV5W9uYEdYUVa79Axb7Rh")
func parseSpotifyURL(raw string) (kind, id string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("spotify: invalid url %q: %w", raw, err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("spotify: cannot extract type and id from %q", raw)
	}
	return parts[0], parts[1], nil
}

// accessToken returns a valid Bearer token, refreshing from Spotify if expired.
func (f *SpotifyFetcher) accessToken(ctx context.Context) (string, error) {
	f.tokenMu.Lock()
	defer f.tokenMu.Unlock()

	if f.token != "" && time.Now().Before(f.tokenExp) {
		return f.token, nil
	}

	body := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", body)
	if err != nil {
		return "", fmt.Errorf("spotify: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(f.clientID, f.clientSecret)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("spotify: token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("spotify: token request returned %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("spotify: decode token response: %w", err)
	}

	// Shave 30 seconds off the expiry to avoid using a token right as it expires.
	f.token = result.AccessToken
	f.tokenExp = time.Now().Add(time.Duration(result.ExpiresIn-30) * time.Second)
	return f.token, nil
}

// spotifyTrack holds only the fields we need from the Spotify track endpoint.
type spotifyTrack struct {
	Name    string `json:"name"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Name   string `json:"name"`
		Images []struct {
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"images"`
	} `json:"album"`
	DurationMS int    `json:"duration_ms"`
	PreviewURL string `json:"preview_url"`
}

func (f *SpotifyFetcher) fetchTrack(ctx context.Context, token, id string) (*spotifyTrack, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.spotify.com/v1/tracks/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("spotify: build track request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spotify: track request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("spotify: track request returned %d: %s", resp.StatusCode, b)
	}

	var track spotifyTrack
	if err := json.NewDecoder(resp.Body).Decode(&track); err != nil {
		return nil, fmt.Errorf("spotify: decode track response: %w", err)
	}
	return &track, nil
}

func (f *SpotifyFetcher) deezerTrackID(ctx context.Context, title, artist string, durationMS int32) string {
	return deezerTrackID(ctx, f.httpClient, title, artist, durationMS)
}
