package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// MyAnimeListFetcher implements Fetcher for myanimelist.net URLs.
// Metadata is resolved via the Jikan v4 public API (api.jikan.moe) which
// requires no API key.
type MyAnimeListFetcher struct {
	httpClient *http.Client
}

func NewMyAnimeListFetcher() *MyAnimeListFetcher {
	return &MyAnimeListFetcher{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

var reMALAnimeID = regexp.MustCompile(`/anime/(\d+)`)
var reMALMangaID = regexp.MustCompile(`/manga/(\d+)`)

func (f *MyAnimeListFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	if m := reMALAnimeID.FindStringSubmatch(resourceURL); len(m) == 2 {
		return f.fetchAnime(ctx, m[1], resourceURL)
	}
	if m := reMALMangaID.FindStringSubmatch(resourceURL); len(m) == 2 {
		return f.fetchManga(ctx, m[1], resourceURL)
	}
	return nil, fmt.Errorf("myanimelist: could not extract ID from %q", resourceURL)
}

func (f *MyAnimeListFetcher) fetchAnime(ctx context.Context, id, resourceURL string) (json.RawMessage, error) {
	var result struct {
		Data struct {
			Images struct {
				JPG struct {
					LargeImageURL string `json:"large_image_url"`
				} `json:"jpg"`
			} `json:"images"`
			Title   string `json:"title"`
			Aired   struct {
				Prop struct {
					From struct {
						Year int32 `json:"year"`
					} `json:"from"`
				} `json:"prop"`
			} `json:"aired"`
			Studios  []struct{ Name string `json:"name"` } `json:"studios"`
			Episodes int32  `json:"episodes"`
			URL      string `json:"url"`
		} `json:"data"`
	}

	if err := f.jikanGet(ctx, "https://api.jikan.moe/v4/anime/"+id, &result); err != nil {
		return nil, fmt.Errorf("myanimelist: jikan anime: %w", err)
	}

	studio := ""
	if len(result.Data.Studios) > 0 {
		studio = result.Data.Studios[0].Name
	}
	sourceURL := result.Data.URL
	if sourceURL == "" {
		sourceURL = resourceURL
	}

	meta := &resourcev1.AnimeMetadata{
		CoverUrl:    result.Data.Images.JPG.LargeImageURL,
		Title:       result.Data.Title,
		ReleaseYear: result.Data.Aired.Prop.From.Year,
		Studio:      studio,
		EpisodeCount: result.Data.Episodes,
		SourceUrl:   sourceURL,
	}
	return protojson.Marshal(meta)
}

func (f *MyAnimeListFetcher) fetchManga(ctx context.Context, id, resourceURL string) (json.RawMessage, error) {
	var result struct {
		Data struct {
			Images struct {
				JPG struct {
					LargeImageURL string `json:"large_image_url"`
				} `json:"jpg"`
			} `json:"images"`
			Title    string `json:"title"`
			Published struct {
				Prop struct {
					From struct {
						Year int32 `json:"year"`
					} `json:"from"`
				} `json:"prop"`
			} `json:"published"`
			Serializations []struct{ Name string `json:"name"` } `json:"serializations"`
			Chapters       int32  `json:"chapters"`
			URL            string `json:"url"`
		} `json:"data"`
	}

	if err := f.jikanGet(ctx, "https://api.jikan.moe/v4/manga/"+id, &result); err != nil {
		return nil, fmt.Errorf("myanimelist: jikan manga: %w", err)
	}

	studio := ""
	if len(result.Data.Serializations) > 0 {
		studio = result.Data.Serializations[0].Name
	}
	sourceURL := result.Data.URL
	if sourceURL == "" {
		sourceURL = resourceURL
	}

	meta := &resourcev1.AnimeMetadata{
		CoverUrl:     result.Data.Images.JPG.LargeImageURL,
		Title:        result.Data.Title,
		ReleaseYear:  result.Data.Published.Prop.From.Year,
		Studio:       studio,
		EpisodeCount: result.Data.Chapters,
		SourceUrl:    sourceURL,
	}
	return protojson.Marshal(meta)
}

func (f *MyAnimeListFetcher) jikanGet(ctx context.Context, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jikan returned %d: %s", resp.StatusCode, b)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
