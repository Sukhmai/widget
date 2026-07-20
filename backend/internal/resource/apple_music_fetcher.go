package resource

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// AppleMusicFetcher implements Fetcher for music.apple.com track URLs using
// the MusicKit catalog API. A developer token is generated once at construction
// and reused for the lifetime of the process (tokens are valid for up to 6 months).
type AppleMusicFetcher struct {
	token      string
	httpClient *http.Client
}

func NewAppleMusicFetcher(teamID, keyID, privateKeyPEM string) (*AppleMusicFetcher, error) {
	token, err := generateAppleMusicToken(teamID, keyID, privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("apple music: generate developer token: %w", err)
	}
	return &AppleMusicFetcher{
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Fetch implements Fetcher.
func (f *AppleMusicFetcher) Fetch(ctx context.Context, resourceURL string) (json.RawMessage, error) {
	storefront, songID, err := parseAppleMusicURL(resourceURL)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("https://api.music.apple.com/v1/catalog/%s/songs/%s", storefront, songID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("apple music: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("apple music: catalog request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("apple music: catalog returned %d: %s", resp.StatusCode, b)
	}

	var body struct {
		Data []struct {
			Attributes struct {
				Name              string `json:"name"`
				ArtistName        string `json:"artistName"`
				AlbumName         string `json:"albumName"`
				DurationInMillis  int32  `json:"durationInMillis"`
				Artwork           struct {
					URL string `json:"url"`
				} `json:"artwork"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("apple music: decode response: %w", err)
	}
	if len(body.Data) == 0 {
		return nil, fmt.Errorf("apple music: no data in catalog response")
	}

	attr := body.Data[0].Attributes
	coverURL := strings.NewReplacer("{w}", "500", "{h}", "500").Replace(attr.Artwork.URL)
	deezerID := deezerTrackID(ctx, f.httpClient, attr.Name, attr.ArtistName, attr.DurationInMillis)

	meta := &resourcev1.MusicTrackMetadata{
		CoverUrl:      coverURL,
		Title:         attr.Name,
		Artist:        attr.ArtistName,
		Album:         attr.AlbumName,
		DurationMs:    attr.DurationInMillis,
		DeezerTrackId: deezerID,
	}
	return protojson.Marshal(meta)
}

// parseAppleMusicURL extracts the storefront and song ID from a music.apple.com URL.
// e.g. https://music.apple.com/us/song/cruel-summer/1468058171 → ("us", "1468058171")
func parseAppleMusicURL(rawURL string) (storefront, songID string, err error) {
	// Path: /us/song/cruel-summer/1468058171
	idx := strings.Index(rawURL, "music.apple.com/")
	if idx == -1 {
		return "", "", fmt.Errorf("apple music: not a music.apple.com URL: %q", rawURL)
	}
	path := strings.TrimPrefix(rawURL[idx:], "music.apple.com/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// parts: ["us", "song", "cruel-summer", "1468058171"]
	if len(parts) < 4 || parts[1] != "song" {
		return "", "", fmt.Errorf("apple music: cannot parse storefront/song-id from %q", rawURL)
	}
	return parts[0], parts[len(parts)-1], nil
}

// generateAppleMusicToken creates an ES256 signed JWT for the MusicKit API.
func generateAppleMusicToken(teamID, keyID, privateKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block from private key")
	}
	raw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse PKCS8 private key: %w", err)
	}
	ecKey, ok := raw.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not ECDSA")
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": teamID,
		"iat": now.Unix(),
		"exp": now.Add(180 * 24 * time.Hour).Unix(), // 6-month max
	})
	token.Header["kid"] = keyID

	return token.SignedString(ecKey)
}
