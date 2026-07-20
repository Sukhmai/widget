package resource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// AniListFetcher implements Fetcher for anilist.co URLs.
// Metadata is resolved via the AniList GraphQL API (graphql.anilist.co) which
// is public and requires no API key.
type AniListFetcher struct {
	httpClient *http.Client
}

func NewAniListFetcher() *AniListFetcher {
	return &AniListFetcher{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

var reAniListAnimeID = regexp.MustCompile(`/anime/(\d+)`)
var reAniListMangaID = regexp.MustCompile(`/manga/(\d+)`)

const anilistGraphQLURL = "https://graphql.anilist.co"

const anilistQuery = `
query ($id: Int, $type: MediaType) {
  Media(id: $id, type: $type) {
    title { english romaji }
    coverImage { large }
    startDate { year }
    studios(isMain: true) { nodes { name } }
    episodes
    chapters
    siteUrl
  }
}`

func (f *AniListFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	var id int
	var mediaType string

	if m := reAniListAnimeID.FindStringSubmatch(resourceURL); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		id, mediaType = n, "ANIME"
	} else if m := reAniListMangaID.FindStringSubmatch(resourceURL); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		id, mediaType = n, "MANGA"
	} else {
		return nil, fmt.Errorf("anilist: could not extract ID from %q", resourceURL)
	}

	body, err := json.Marshal(map[string]any{
		"query":     anilistQuery,
		"variables": map[string]any{"id": id, "type": mediaType},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anilistGraphQLURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anilist: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anilist: returned %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Data struct {
			Media struct {
				Title struct {
					English string `json:"english"`
					Romaji  string `json:"romaji"`
				} `json:"title"`
				CoverImage struct {
					Large string `json:"large"`
				} `json:"coverImage"`
				StartDate struct {
					Year int32 `json:"year"`
				} `json:"startDate"`
				Studios struct {
					Nodes []struct {
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"studios"`
				Episodes *int32 `json:"episodes"`
				Chapters *int32 `json:"chapters"`
				SiteURL  string `json:"siteUrl"`
			} `json:"Media"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("anilist: decode: %w", err)
	}

	m := result.Data.Media

	title := m.Title.English
	if title == "" {
		title = m.Title.Romaji
	}

	studio := ""
	if len(m.Studios.Nodes) > 0 {
		studio = m.Studios.Nodes[0].Name
	}

	var count int32
	if m.Episodes != nil {
		count = *m.Episodes
	} else if m.Chapters != nil {
		count = *m.Chapters
	}

	sourceURL := m.SiteURL
	if sourceURL == "" {
		sourceURL = resourceURL
	}

	meta := &resourcev1.AnimeMetadata{
		CoverUrl:     m.CoverImage.Large,
		Title:        title,
		ReleaseYear:  m.StartDate.Year,
		Studio:       studio,
		EpisodeCount: count,
		SourceUrl:    sourceURL,
	}
	return protojson.Marshal(meta)
}
