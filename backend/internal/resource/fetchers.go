package resource

// FetcherConfig holds the credentials needed to construct the full set of
// registered fetchers. Shared between cmd/worker (async queue processing)
// and cmd/api (synchronous URL preview) so the two entrypoints can't drift
// on which sources have a fetcher.
type FetcherConfig struct {
	SpotifyClientID     string
	SpotifyClientSecret string
	TMDBAccessToken     string
}

// BuildFetchers constructs every registered Fetcher, keyed by Source.
func BuildFetchers(cfg FetcherConfig) map[Source]Fetcher {
	return map[Source]Fetcher{
		SourceSpotify:     NewSpotifyFetcher(cfg.SpotifyClientID, cfg.SpotifyClientSecret),
		SourceLetterboxd:  NewLetterboxdFetcher(cfg.TMDBAccessToken),
		SourceIMDB:        NewIMDBFetcher(cfg.TMDBAccessToken),
		SourceGoodreads:   NewGoodreadsFetcher(),
		SourceX:           NewXFetcher(),
		SourceYouTube:     NewYouTubeFetcher(),
		SourceWikipedia:   NewWikipediaFetcher(),
		SourceSteam:       NewSteamFetcher(),
		SourceSubstack:    NewSubstackFetcher(),
		SourceReddit:      NewRedditFetcher(),
		SourceTikTok:      NewTikTokFetcher(),
		SourceMyAnimeList: NewMyAnimeListFetcher(),
		SourceAniList:     NewAniListFetcher(),
		SourceBluesky:     NewBlueskyFetcher(),
		SourceImage:       NewImageFetcher(),
		SourceGeneric:     NewGenericFetcher(),
	}
}
