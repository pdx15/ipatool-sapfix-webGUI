package appstore

import (
	"github.com/majd/ipatool/v2/pkg/anisette"
	"github.com/majd/ipatool/v2/pkg/gsa"
	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/keychain"
	"github.com/majd/ipatool/v2/pkg/mescal"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	"github.com/majd/ipatool/v2/pkg/util/operatingsystem"
)

type AppStore interface {
	// Login authenticates with the App Store.
	Login(input LoginInput) (LoginOutput, error)
	// LoginMZFinance authenticates with the App Store using the stable legacy
	// MZFinance authenticate endpoint: it runs the GSA (SRP-6a) handshake
	// first (for public anisette and two-factor handling), then completes
	// authentication via the MZFinance endpoint — bypassing the glitchy
	// native/fast fallback that Login may use on macOS. It is a diagnostic
	// alternative to Login and does not change Login's behaviour.
	LoginMZFinance(input LoginInput) (LoginOutput, error)
	// AccountInfo returns the information of the authenticated account.
	AccountInfo() (AccountInfoOutput, error)
	// ImportAccount stores an existing account session, e.g. one exported on
	// another machine using "ipatool auth export".
	ImportAccount(input ImportAccountInput) (ImportAccountOutput, error)
	// Revoke revokes the active credentials.
	Revoke() error
	// Lookup looks apps up based on the specified bundle identifier.
	Lookup(input LookupInput) (LookupOutput, error)
	// Search searches the App Store for apps matching the specified term.
	Search(input SearchInput) (SearchOutput, error)
	// Purchase acquires a license for the desired app.
	// Note: only free apps are supported.
	Purchase(input PurchaseInput) error
	// CheckDownload performs the direct-download request without transferring
	// the package, so callers can validate that the account holds a license
	// (it returns ErrLicenseRequired otherwise).
	CheckDownload(input CheckDownloadInput) (CheckDownloadOutput, error)
	// Download downloads the IPA package from the App Store to the desired location.
	Download(input DownloadInput) (DownloadOutput, error)
	// ReplicateSinf replicates the sinf for the IPA package.
	ReplicateSinf(input ReplicateSinfInput) error
	// VersionHistory lists the available versions of the specified app.
	ListVersions(input ListVersionsInput) (ListVersionsOutput, error)
	// GetVersionMetadata returns the metadata for the specified version.
	GetVersionMetadata(input GetVersionMetadataInput) (GetVersionMetadataOutput, error)
	// Bag fetches the bag which contains endpoint definitions.
	Bag(input BagInput) (BagOutput, error)
	// OwnedApps lists the apps owned by the authenticated account (purchase
	// history), newest purchase first.
	OwnedApps(input OwnedAppsInput) (OwnedAppsOutput, error)
}

// gsaClient is the subset of gsa.Client used by the appstore login flow. It
// is an interface so the GSA path can be exercised in tests without a live
// SRP handshake.
type gsaClient interface {
	Login(email, password string, anisette anisette.Data, authCode string) (gsa.Account, error)
	ItunesAuthenticate(account gsa.Account, anisette anisette.Data, guid string) (gsa.Account, error)
}

type appstore struct {
	keychain        keychain.Keychain
	cookieJar       http.CookieJar
	loginClient     http.Client[loginResult]
	searchClient    http.Client[searchResult]
	purchaseClient  http.Client[purchaseResult]
	downloadClient  http.Client[downloadResult]
	platformClient  http.Client[platformVersionLookupResult]
	bagClient       http.Client[bagResult]
	ownedAppsClient http.Client[[]byte]
	httpClient      http.Client[interface{}]
	machine         machine.Machine
	os              operatingsystem.OperatingSystem
	gsa             gsaClient
	anisette        anisette.Provider
}

type Args struct {
	Keychain        keychain.Keychain
	CookieJar       http.CookieJar
	OperatingSystem operatingsystem.OperatingSystem
	Machine         machine.Machine
}

func NewAppStore(args Args) AppStore {
	clientArgs := http.Args{
		CookieJar:    args.CookieJar,
		ActionSigner: mescal.Sign,
	}

	return &appstore{
		keychain:        args.Keychain,
		cookieJar:       args.CookieJar,
		loginClient:     http.NewClient[loginResult](clientArgs),
		searchClient:    http.NewClient[searchResult](clientArgs),
		purchaseClient:  http.NewClient[purchaseResult](clientArgs),
		downloadClient:  http.NewClient[downloadResult](clientArgs),
		platformClient:  http.NewClient[platformVersionLookupResult](clientArgs),
		bagClient:       http.NewClient[bagResult](clientArgs),
		ownedAppsClient: http.NewClient[[]byte](clientArgs),
		httpClient:      http.NewClient[interface{}](clientArgs),
		machine:         args.Machine,
		os:              args.OperatingSystem,
		gsa:             gsa.NewClient(args.CookieJar),
		anisette:        anisette.NewProvider(nil),
	}
}
