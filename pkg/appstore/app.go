package appstore

import (
	"github.com/rs/zerolog"
)

type App struct {
	ID             int64    `json:"trackId,omitempty"`
	BundleID       string   `json:"bundleId,omitempty"`
	Name           string   `json:"trackName,omitempty"`
	Version        string   `json:"version,omitempty"`
	Price          float64  `json:"price,omitempty"`
	ArtworkURL60   string   `json:"artworkUrl60,omitempty"`
	ArtworkURL100  string   `json:"artworkUrl100,omitempty"`
	ArtworkURL512  string   `json:"artworkUrl512,omitempty"`
	ArtistName     string   `json:"artistName,omitempty"`
	SellerName     string   `json:"sellerName,omitempty"`
	FormattedPrice string   `json:"formattedPrice,omitempty"`
	Description    string   `json:"description,omitempty"`
	AverageRating  float64  `json:"averageUserRating,omitempty"`
	RatingCount    int64    `json:"userRatingCount,omitempty"`
	Genres         []string `json:"genres,omitempty"`
}

type VersionHistoryInfo struct {
	App                App
	LatestVersion      string
	VersionIdentifiers []string
}

type VersionDetails struct {
	VersionID     string
	VersionString string
	Success       bool
	Error         string
}

type Apps []App

func (apps Apps) MarshalZerologArray(a *zerolog.Array) {
	for _, app := range apps {
		a.Object(app)
	}
}

func (a App) MarshalZerologObject(event *zerolog.Event) {
	event.
		Int64("id", a.ID).
		Str("bundleID", a.BundleID).
		Str("name", a.Name).
		Str("version", a.Version).
		Float64("price", a.Price)
}
