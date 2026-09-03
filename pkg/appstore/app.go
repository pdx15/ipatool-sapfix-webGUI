package appstore

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

// FlexibleInt64 accepts both a JSON number and a JSON string when decoding.
// Apple's search/lookup responses sometimes return numeric fields (e.g.
// fileSizeBytes) as a quoted string; this type keeps the request working
// regardless of which form Apple returns.
type FlexibleInt64 int64

func (f *FlexibleInt64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*f = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*f = FlexibleInt64(v)
		return nil
	}
	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*f = FlexibleInt64(v)
	return nil
}

type App struct {
	ID             int64         `json:"trackId,omitempty"`
	BundleID       string        `json:"bundleId,omitempty"`
	Name           string        `json:"trackName,omitempty"`
	Version        string        `json:"version,omitempty"`
	FileSizeBytes  FlexibleInt64 `json:"fileSizeBytes,omitempty"`
	Price          float64       `json:"price,omitempty"`
	ArtworkURL60   string        `json:"artworkUrl60,omitempty"`
	ArtworkURL100  string        `json:"artworkUrl100,omitempty"`
	ArtworkURL512  string        `json:"artworkUrl512,omitempty"`
	ArtistName     string        `json:"artistName,omitempty"`
	SellerName     string        `json:"sellerName,omitempty"`
	FormattedPrice string        `json:"formattedPrice,omitempty"`
	Description    string        `json:"description,omitempty"`
	AverageRating  float64       `json:"averageUserRating,omitempty"`
	RatingCount    int64         `json:"userRatingCount,omitempty"`
	Genres         []string      `json:"genres,omitempty"`
	// PurchaseDate is only populated for apps returned by OwnedApps.
	PurchaseDate time.Time `json:"purchaseDate,omitzero"`
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

	if !a.PurchaseDate.IsZero() {
		event.Time("purchaseDate", a.PurchaseDate)
	}
}
