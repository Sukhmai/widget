package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// deezerTrackID searches the Deezer catalog for a track matching title + artist
// and returns its Deezer track ID. The match is duration-gated (±3 s) to avoid
// false positives. Returns empty string when no confident match is found.
func deezerTrackID(ctx context.Context, client *http.Client, title, artist string, durationMS int32) string {
	q := `artist:"` + artist + `" track:"` + title + `"`
	apiURL := "https://api.deezer.com/search/track?q=" + url.QueryEscape(q) + "&limit=5"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var result struct {
		Data []struct {
			ID       int64  `json:"id"`
			Preview  string `json:"preview"`
			Duration int32  `json:"duration"` // seconds
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	durationSec := durationMS / 1000
	for _, item := range result.Data {
		if item.Preview == "" {
			continue
		}
		diff := item.Duration - durationSec
		if diff < 0 {
			diff = -diff
		}
		if diff <= 3 {
			return fmt.Sprintf("%d", item.ID)
		}
	}
	return ""
}
