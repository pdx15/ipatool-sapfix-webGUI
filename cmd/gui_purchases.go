package cmd

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/majd/ipatool/v2/pkg/appstore"
)

// purchasesCache keeps the last purchase-history result per Apple ID so the
// "Purchase history" tab is populated instantly after the first load. The
// client asks for a fresh copy explicitly with ?refresh=1.
type purchasesCache struct {
	sync.Mutex
	dsid      string
	apps      []appstore.App
	fetchedAt time.Time
	// inflight collapses concurrent loads (e.g. a refresh click while the
	// initial load is still running) into one Apple round-trip.
	inflight *purchasesLoad
}

type purchasesLoad struct {
	done chan struct{}
	apps []appstore.App
	err  error
}

var purchasesState = &purchasesCache{}

// purchaseItem is the JSON shape consumed by the GUI.
type purchaseItem struct {
	AppID        int64  `json:"appId"`
	BundleID     string `json:"bundleId"`
	Name         string `json:"name"`
	Version      string `json:"version,omitempty"`
	PurchaseDate string `json:"purchaseDate,omitempty"`
}

// ownedAppsWithRetry loads the full purchase history and refreshes the stored
// session once when Apple reports an expired password token.
//
//nolint:wrapcheck
func ownedAppsWithRetry(acc appstore.Account) (appstore.Account, []appstore.App, error) {
	refreshed := false

	for attempt := 0; attempt < 2; attempt++ {
		out, err := dependencies.AppStore.OwnedApps(appstore.OwnedAppsInput{
			Account: acc,
			All:     true,
		})
		if err == nil {
			return acc, out.Results, nil
		}

		if errors.Is(err, appstore.ErrPasswordTokenExpired) && !refreshed {
			if acc.Password == "" {
				return acc, nil, errors.New("password token is expired and no password is stored; log in again or import a fresh account session")
			}

			refreshedAcc, refreshErr := refreshAccount(acc)
			if refreshErr != nil {
				return acc, nil, refreshErr
			}

			acc = refreshedAcc
			refreshed = true
			continue
		}

		return acc, nil, err
	}

	return acc, nil, errors.New("failed to load purchase history after re-authentication")
}

// loadPurchases returns the cached purchase list for acc, loading it from
// Apple when the cache is empty, belongs to another account, or force is set.
func loadPurchases(acc appstore.Account, force bool) ([]appstore.App, time.Time, bool, error) {
	purchasesState.Lock()

	if !force && purchasesState.dsid == acc.DirectoryServicesID && purchasesState.apps != nil {
		apps, at := purchasesState.apps, purchasesState.fetchedAt
		purchasesState.Unlock()
		return apps, at, true, nil
	}

	if load := purchasesState.inflight; load != nil {
		purchasesState.Unlock()
		<-load.done
		return load.apps, time.Now(), false, load.err
	}

	load := &purchasesLoad{done: make(chan struct{})}
	purchasesState.inflight = load
	purchasesState.Unlock()

	_, apps, err := ownedAppsWithRetry(acc)

	purchasesState.Lock()
	purchasesState.inflight = nil
	now := time.Now()
	if err == nil {
		purchasesState.dsid = acc.DirectoryServicesID
		purchasesState.apps = apps
		purchasesState.fetchedAt = now
	}
	purchasesState.Unlock()

	load.apps, load.err = apps, err
	close(load.done)

	return apps, now, false, err
}

// invalidatePurchasesCache drops the cached list, e.g. after logout.
func invalidatePurchasesCache() {
	purchasesState.Lock()
	purchasesState.dsid = ""
	purchasesState.apps = nil
	purchasesState.fetchedAt = time.Time{}
	purchasesState.Unlock()
}

// handleAPIPurchases serves GET /api/purchases[?refresh=1].
func handleAPIPurchases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "you must log in to view purchase history")
		return
	}

	force := r.URL.Query().Get("refresh") == "1"

	apps, fetchedAt, cached, err := loadPurchases(info.Account, force)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, appstore.ErrPasswordTokenExpired) {
			status = http.StatusUnauthorized
		}
		jsonError(w, status, err.Error())
		return
	}

	items := make([]purchaseItem, 0, len(apps))
	for _, app := range apps {
		item := purchaseItem{
			AppID:    app.ID,
			BundleID: app.BundleID,
			Name:     app.Name,
			Version:  app.Version,
		}
		if !app.PurchaseDate.IsZero() {
			item.PurchaseDate = app.PurchaseDate.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"cached":    cached,
		"fetchedAt": fetchedAt.UTC().Format(time.RFC3339),
		"count":     len(items),
		"apps":      items,
	})
}
