package resource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// ImageFetcher implements Fetcher for URLs that point directly at an image
// file (DetectSource matched by extension). It never errors — a HEAD request
// is only used best-effort to resolve redirects; if it fails, the metadata
// still carries the original URL so the card renders instead of dead-ending
// in a perpetual loading state.
type ImageFetcher struct {
	httpClient *http.Client
}

func NewImageFetcher() *ImageFetcher {
	return &ImageFetcher{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (f *ImageFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	finalURL := resourceURL

	if req, err := http.NewRequestWithContext(ctx, http.MethodHead, resourceURL, nil); err == nil {
		req.Header.Set("User-Agent", browserUA)
		if resp, err := f.httpClient.Do(req); err == nil {
			resp.Body.Close()
			if resp.Request != nil && resp.Request.URL != nil {
				finalURL = resp.Request.URL.String()
			}
		}
	}

	siteName := ""
	if base, err := url.Parse(finalURL); err == nil && base.Hostname() != "" {
		siteName = prettyHostName(base.Hostname())
	}

	out := &resourcev1.ImageMetadata{
		ImageUrl: finalURL,
		SiteName: siteName,
	}

	return protojson.Marshal(out)
}
