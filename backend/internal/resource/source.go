package resource

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Source identifies where a resource originates.
// Validation lives here at the application layer; the DB stores a plain TEXT column.
type Source string

const (
	SourceGeneric    Source = "generic"
	SourceSpotify    Source = "spotify"
	SourceYouTube    Source = "youtube"
	SourceLetterboxd Source = "letterboxd"
	SourceGoodreads  Source = "goodreads"
	SourceX          Source = "x"
	SourceWikipedia  Source = "wikipedia"
	SourceSteam      Source = "steam"
	SourceSubstack   Source = "substack"
	SourceAppleMusic  Source = "apple_music"
	SourceIMDB        Source = "imdb"
	SourceReddit      Source = "reddit"
	SourceTikTok      Source = "tiktok"
	SourceMyAnimeList Source = "myanimelist"
	SourceAniList     Source = "anilist"
	SourceBluesky     Source = "bluesky"
	SourceInstagram   Source = "instagram"
	SourceImage       Source = "image"
)

// knownSources is the single allow-list that drives parseSource. Add new
// Source constants here to keep them in sync.
var knownSources = []Source{
	SourceGeneric, SourceSpotify, SourceYouTube, SourceLetterboxd, SourceGoodreads, SourceX, SourceWikipedia, SourceSteam, SourceSubstack, SourceAppleMusic, SourceIMDB, SourceReddit, SourceTikTok, SourceMyAnimeList, SourceAniList, SourceBluesky, SourceInstagram, SourceImage,
}

// reImageExt matches a direct link to an image file by its extension, ignoring
// any query string (e.g. CDN cache-busting params). This mirrors how the
// frontend's URL_PATTERNS would classify a pasted image link.
var reImageExt = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|webp|avif|bmp|heic|heif)(\?.*)?$`)

// DetectSource derives a Source from a URL. Always returns a valid Source,
// falling back to SourceGeneric for unrecognised hosts. Path guards mirror the
// frontend URL_PATTERNS so that non-canonical pages (profiles, lists, etc.) on
// recognised domains fall through to generic rather than reaching a fetcher
// that can't parse them. Short-link domains (boxd.it, redd.it, t.co, s.team,
// vm.tiktok.com) carry no path signal and are matched by host alone.
func DetectSource(rawURL string) Source {
	switch {
	case strings.Contains(rawURL, "open.spotify.com") && strings.Contains(rawURL, "/track/"):
		return SourceSpotify
	case isYouTubeVideoURL(rawURL):
		return SourceYouTube
	case (strings.Contains(rawURL, "letterboxd.com") && strings.Contains(rawURL, "/film/")) ||
		strings.Contains(rawURL, "boxd.it"):
		return SourceLetterboxd
	case strings.Contains(rawURL, "goodreads.com") && strings.Contains(rawURL, "/book/show/"):
		return SourceGoodreads
	case (strings.Contains(rawURL, "twitter.com") || strings.Contains(rawURL, "x.com")) && strings.Contains(rawURL, "/status/") ||
		strings.Contains(rawURL, "//t.co/"):
		return SourceX
	case strings.Contains(rawURL, ".wikipedia.org") && strings.Contains(rawURL, "/wiki/"):
		return SourceWikipedia
	case (strings.Contains(rawURL, "store.steampowered.com") && strings.Contains(rawURL, "/app/")) ||
		strings.Contains(rawURL, "s.team"):
		return SourceSteam
	case strings.Contains(rawURL, ".substack.com") && strings.Contains(rawURL, "/p/"):
		return SourceSubstack
	// case strings.Contains(rawURL, "music.apple.com") && strings.Contains(rawURL, "/song/"):
	// 	return SourceAppleMusic
	case strings.Contains(rawURL, "imdb.com") && strings.Contains(rawURL, "/title/tt"):
		return SourceIMDB
	// case (strings.Contains(rawURL, "reddit.com") && strings.Contains(rawURL, "/comments/")) ||
	// 	strings.Contains(rawURL, "redd.it"):
	// 	return SourceReddit
	case (strings.Contains(rawURL, "tiktok.com") && strings.Contains(rawURL, "/video/")) ||
		strings.Contains(rawURL, "vm.tiktok.com"):
		return SourceTikTok
	case strings.Contains(rawURL, "myanimelist.net") &&
		(strings.Contains(rawURL, "/anime/") || strings.Contains(rawURL, "/manga/")):
		return SourceMyAnimeList
	case strings.Contains(rawURL, "anilist.co") &&
		(strings.Contains(rawURL, "/anime/") || strings.Contains(rawURL, "/manga/")):
		return SourceAniList
	case strings.Contains(rawURL, "bsky.app") && strings.Contains(rawURL, "/post/"):
		return SourceBluesky
	// case (strings.Contains(rawURL, "instagram.com") &&
	// 	(strings.Contains(rawURL, "/p/") || strings.Contains(rawURL, "/reel/"))) ||
	// 	strings.Contains(rawURL, "instagr.am"):
	// 	return SourceInstagram
	case reImageExt.MatchString(rawURL):
		return SourceImage
	default:
		return SourceGeneric
	}
}

// isYouTubeVideoURL returns true only for video URLs (watch?v=, /shorts/, youtu.be/).
// Channel, playlist, and other youtube.com pages are left to the generic fetcher.
func isYouTubeVideoURL(rawURL string) bool {
	if strings.Contains(rawURL, "youtu.be/") {
		return true
	}
	if !strings.Contains(rawURL, "youtube.com") {
		return false
	}
	return strings.Contains(rawURL, "watch?") && strings.Contains(rawURL, "v=") ||
		strings.Contains(rawURL, "/shorts/")
}

// parseSource is used internally by the worker when reading a source value
// back from the DB. Returns an error if the stored value is unrecognised —
// that indicates a deployment skew worth surfacing.
func parseSource(s string) (Source, error) {
	src := Source(s)
	if slices.Contains(knownSources, src) {
		return src, nil
	}
	return "", fmt.Errorf("resource: unknown source %q stored in DB", s)
}
