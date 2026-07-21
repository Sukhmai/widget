package atom

import (
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sukhmai/widget/backend/internal/db"
	atomv1 "github.com/sukhmai/widget/backend/internal/gen/atom/v1"
	resourcev1 "github.com/sukhmai/widget/backend/internal/gen/resource/v1"
	"github.com/sukhmai/widget/backend/internal/resource"
	"google.golang.org/protobuf/encoding/protojson"
)

const timeFormat = "2006-01-02T15:04:05Z"

// parseUUID parses a canonical UUID string into a pgtype.UUID.
func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if s == "" {
		return u, errors.New("empty uuid")
	}
	if err := u.Scan(s); err != nil {
		return u, err
	}
	return u, nil
}

// resourceToProto builds a proto Resource from the denormalised columns that
// both GetAtomRow and ListAtomsByUserRow expose. When fetched is true it
// unmarshals the stored JSON into the correct metadata oneof case based on
// source; unrecognised sources leave metadata unset.
func resourceToProto(id, url, source string, fetched bool, metadata []byte) *atomv1.Resource {
	r := &atomv1.Resource{
		Id:      id,
		Url:     url,
		Source:  source,
		Fetched: fetched,
	}
	if !fetched || len(metadata) == 0 {
		return r
	}
	switch resource.Source(source) {
	case resource.SourceSpotify:
		var m resourcev1.MusicTrackMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_SpotifyTrack{SpotifyTrack: &m}
		}
	case resource.SourceLetterboxd:
		var m resourcev1.MovieMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_Movie{Movie: &m}
		}
	case resource.SourceGoodreads:
		var m resourcev1.BookMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_Book{Book: &m}
		}
	case resource.SourceX:
		var m resourcev1.TweetMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_Tweet{Tweet: &m}
		}
	case resource.SourceWikipedia:
		var m resourcev1.WikipediaMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_Wikipedia{Wikipedia: &m}
		}
	case resource.SourceSteam:
		var m resourcev1.SteamGameMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_SteamGame{SteamGame: &m}
		}
	case resource.SourceSubstack:
		var m resourcev1.SubstackPostMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_SubstackPost{SubstackPost: &m}
		}
	case resource.SourceYouTube:
		var m resourcev1.YouTubeVideoMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_YoutubeVideo{YoutubeVideo: &m}
		}
	case resource.SourceIMDB:
		var m resourcev1.MovieMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_ImdbMovie{ImdbMovie: &m}
		}
	case resource.SourceReddit:
		var m resourcev1.RedditPostMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_RedditPost{RedditPost: &m}
		}
	case resource.SourceTikTok:
		var m resourcev1.TikTokVideoMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_TiktokVideo{TiktokVideo: &m}
		}
	case resource.SourceMyAnimeList:
		var m resourcev1.AnimeMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_MalAnime{MalAnime: &m}
		}
	case resource.SourceAniList:
		var m resourcev1.AnimeMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_AnilistAnime{AnilistAnime: &m}
		}
	case resource.SourceBluesky:
		var m resourcev1.BlueskyPostMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_BlueskyPost{BlueskyPost: &m}
		}
	case resource.SourceInstagram:
		var m resourcev1.InstagramPostMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_InstagramPost{InstagramPost: &m}
		}
	case resource.SourceGeneric:
		var m resourcev1.GenericMetadata
		if err := protojson.Unmarshal(metadata, &m); err == nil {
			r.Metadata = &atomv1.Resource_Generic{Generic: &m}
		}
	}
	return r
}

// getAtomRowToProto converts a GetAtomRow (atom + left-joined resource) into
// the proto Atom message.
func getAtomRowToProto(row *db.GetAtomRow) *atomv1.Atom {
	a := &atomv1.Atom{
		Id:        row.ID.String(),
		UserId:    row.UserID.String(),
		Text:      row.Text.String,
		CreatedAt: row.CreatedAt.Time.UTC().Format(timeFormat),
	}
	if row.EditedAt.Valid {
		a.EditedAt = row.EditedAt.Time.UTC().Format(timeFormat)
	}
	if row.ResourceID.Valid {
		a.Resource = resourceToProto(
			row.ResourceID.String(),
			row.ResourceUrl.String,
			row.ResourceSource.String,
			row.ResourceFetchedAt.Valid,
			row.ResourceMetadata,
		)
	}
	return a
}

// latestAtomRowToProto converts a GetLatestAtomFromUserRow into the proto Atom message.
func latestAtomRowToProto(row *db.GetLatestAtomFromUserRow) *atomv1.Atom {
	a := &atomv1.Atom{
		Id:        row.ID.String(),
		UserId:    row.UserID.String(),
		Text:      row.Text.String,
		CreatedAt: row.CreatedAt.Time.UTC().Format(timeFormat),
	}
	if row.EditedAt.Valid {
		a.EditedAt = row.EditedAt.Time.UTC().Format(timeFormat)
	}
	if row.ResourceID.Valid {
		a.Resource = resourceToProto(
			row.ResourceID.String(),
			row.ResourceUrl.String,
			row.ResourceSource.String,
			row.ResourceFetchedAt.Valid,
			row.ResourceMetadata,
		)
	}
	return a
}

// listAtomRowToProto converts a ListAtomsByUserRow into the proto Atom message.
func listAtomRowToProto(row *db.ListAtomsByUserRow) *atomv1.Atom {
	a := &atomv1.Atom{
		Id:        row.ID.String(),
		UserId:    row.UserID.String(),
		Text:      row.Text.String,
		CreatedAt: row.CreatedAt.Time.UTC().Format(timeFormat),
	}
	if row.EditedAt.Valid {
		a.EditedAt = row.EditedAt.Time.UTC().Format(timeFormat)
	}
	if row.ResourceID.Valid {
		a.Resource = resourceToProto(
			row.ResourceID.String(),
			row.ResourceUrl.String,
			row.ResourceSource.String,
			row.ResourceFetchedAt.Valid,
			row.ResourceMetadata,
		)
	}
	return a
}
