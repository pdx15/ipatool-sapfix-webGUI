package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/majd/ipatool/v2/pkg/appstore"
)

// batch item statuses reported by the batch check job.
const (
	batchItemChecking        = "checking"
	batchItemAvailable       = "available"
	batchItemLicenseRequired = "license-required"
	batchItemError           = "error"
	// batchItemStalled marks an item the watchdog gave up on: it stopped the
	// batch from moving forward without ever returning an error.
	batchItemStalled = "stalled"
	// batchItemSkipped marks an item the user skipped by hand.
	batchItemSkipped = "skipped"
)

// Watchdog defaults. A single app that neither errors out nor makes progress
// would block the whole batch forever, so every item is watched and can be
// skipped; the batch then carries on with the next one.
const (
	// defaultStallTimeout is how long a downloading item may stay at exactly
	// the same number of bytes before it counts as hung.
	defaultStallTimeout = 5 * time.Minute
	// defaultItemTimeout is the hard cap for one item and covers the phases
	// without measurable progress (license acquisition, sinf patching).
	defaultItemTimeout = 30 * time.Minute
	// defaultCheckItemTimeout caps one direct-download probe. A probe is a
	// light request, so it needs a much shorter budget than a download.
	defaultCheckItemTimeout = 3 * time.Minute
	// watchdogTick is how often the watchdog samples the live job.
	watchdogTick = time.Second
	// minBatchTimeout guards against unusably small configured values.
	minBatchTimeout = 30 * time.Second
)

// timeoutFromSeconds turns an optional request value into a duration, keeping
// the default when the field is absent or nonsensical.
func timeoutFromSeconds(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}

	d := time.Duration(seconds) * time.Second
	if d < minBatchTimeout {
		return minBatchTimeout
	}

	return d
}

// batchCheckItem is the per-app result of a batch direct-download check.
type batchCheckItem struct {
	AppID                      int64    `json:"appId"`
	Name                       string   `json:"name"`
	Status                     string   `json:"status"`
	Error                      string   `json:"error,omitempty"`
	Version                    string   `json:"version,omitempty"`
	ExternalVersionIdentifiers []string `json:"externalVersionIdentifiers,omitempty"`
	LatestExternalVersionID    string   `json:"latestExternalVersionId,omitempty"`
}

// batchCheckJob tracks an asynchronous run that probes each app through the
// real direct-download request and keeps the apps the account holds licenses
// for. Progress is an overall percentage with equal weight per app.
type batchCheckJob struct {
	ID        string           `json:"id"`
	Status    string           `json:"status"` // "running", "completed"
	Total     int              `json:"total"`
	Done      int              `json:"done"`
	Progress  float64          `json:"progress"`
	Items     []batchCheckItem `json:"items"`
	Error     string           `json:"error,omitempty"`
	CreatedAt int64            `json:"createdAt"`
}

type batchCheckTracker struct {
	sync.RWMutex
	jobs map[string]*batchCheckJob
}

var batchCheckJobs = &batchCheckTracker{
	jobs: make(map[string]*batchCheckJob),
}

// add stores a new job under the tracker lock.
func (t *batchCheckTracker) add(job *batchCheckJob) {
	t.Lock()
	defer t.Unlock()
	t.jobs[job.ID] = job
}

// update runs fn against the stored job while holding the tracker lock so the
// concurrent status endpoint never observes partially updated items.
func (t *batchCheckTracker) update(id string, fn func(*batchCheckJob)) {
	t.Lock()
	defer t.Unlock()
	if job, ok := t.jobs[id]; ok {
		fn(job)
	}
}

func (t *batchCheckTracker) get(id string) (*batchCheckJob, bool) {
	t.RLock()
	defer t.RUnlock()

	job, ok := t.jobs[id]
	if !ok {
		return nil, false
	}

	copyJob := *job
	copyJob.Items = make([]batchCheckItem, len(job.Items))
	copy(copyJob.Items, job.Items)

	return &copyJob, true
}

type batchCheckRequestPayload struct {
	Platform string `json:"platform"`
	// Purchase is a pointer so an API client that omits the field keeps the
	// previous default behaviour (auto-acquiring licenses).
	Purchase *bool `json:"purchase"`
	// ItemTimeoutSec caps one probe. Zero or a missing field keeps the
	// built-in default.
	ItemTimeoutSec int `json:"itemTimeoutSec"`
	Items          []struct {
		AppID int64  `json:"appId"`
		Name  string `json:"name"`
	} `json:"items"`
}

// batchAutoPurchase returns the effective auto-purchase setting for batch
// requests. Omitting the field keeps the original behavior (enabled).
func batchAutoPurchase(purchase *bool) bool {
	if purchase == nil {
		return true
	}

	return *purchase
}

func handleAPIBatchCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req batchCheckRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if len(req.Items) == 0 {
		jsonError(w, http.StatusBadRequest, "at least one app ID is required")
		return
	}

	for _, item := range req.Items {
		if item.AppID == 0 {
			jsonError(w, http.StatusBadRequest, "app ID is required")
			return
		}
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "you must log in to check apps")
		return
	}

	job := &batchCheckJob{
		ID:        fmt.Sprintf("batch_check_%d", time.Now().UnixNano()),
		Status:    "running",
		Total:     len(req.Items),
		Items:     make([]batchCheckItem, len(req.Items)),
		CreatedAt: time.Now().Unix(),
	}

	for i, item := range req.Items {
		job.Items[i] = batchCheckItem{
			AppID: item.AppID,
			Name:  item.Name,
		}
	}

	batchCheckJobs.add(job)

	go executeBatchCheckJob(
		job,
		req.Platform,
		info.Account,
		batchAutoPurchase(req.Purchase),
		timeoutFromSeconds(req.ItemTimeoutSec, defaultCheckItemTimeout),
	)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"jobId":   job.ID,
		"total":   job.Total,
	})
}

// batchCheckResult carries everything one probe produced, including the
// account to keep using for the following apps.
type batchCheckResult struct {
	acc         appstore.Account
	out         appstore.CheckDownloadOutput
	purchaseErr error
	checkErr    error
	// skipped is set when the probe never answered within its time budget.
	skipped bool
}

// runBatchCheckItem probes one app with a hard time budget. A probe is only a
// handful of requests, so a plain timeout is enough here — unlike a download
// there is no byte stream to watch.
func runBatchCheckItem(
	app appstore.App,
	acc appstore.Account,
	platform appstore.Platform,
	purchase bool,
	timeout time.Duration,
) batchCheckResult {
	resCh := make(chan batchCheckResult, 1)

	go func() {
		res := batchCheckResult{acc: acc}

		// When auto-purchase is enabled, attempt to acquire the license (with
		// automatic session refresh when needed) before the direct-download
		// request. The last known account is reused even when the purchase call
		// itself fails, so a refreshed session is still available for the check.
		if purchase {
			refreshed, _, purchaseErr := purchaseWithRetry(acc, app)
			if purchaseErr == nil {
				res.acc = refreshed
			}

			res.purchaseErr = purchaseErr
		}

		_, out, checkErr := checkDownloadWithRetry(res.acc, app, platform)
		res.out = out
		res.checkErr = checkErr

		resCh <- res
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// When the timer wins, the probe is still running in the background. The
	// batch moves on and reports the app as stalled, so a hung app is never
	// silently treated as available.
	select {
	case res := <-resCh:
		return res
	case <-timer.C:
		return batchCheckResult{acc: acc, skipped: true}
	}
}

// executeBatchCheckJob runs every app through the same sequence the Direct
// Download path uses (license acquisition attempt followed by the
// volumeStoreDownloadProduct request, minus the package transfer), filtering
// out apps for which the account has no license. Each finished app advances
// the overall progress by an equal step, so 100 apps mean 1% per pass.
//
// Every probe is time-boxed: an app that never answers would otherwise hold up
// the remaining ones exactly like a hung download does.
func executeBatchCheckJob(
	job *batchCheckJob,
	platformStr string,
	acc appstore.Account,
	purchase bool,
	itemTimeout time.Duration,
) {
	platform, _ := appstore.ParsePlatform(platformStr)

	// Keep a private snapshot of the immutable inputs; the job map itself is
	// only mutated through the tracker lock.
	items := make([]batchCheckItem, len(job.Items))
	copy(items, job.Items)

	for i := range items {
		app := appstore.App{ID: items[i].AppID}

		batchCheckJobs.update(job.ID, func(j *batchCheckJob) {
			j.Items[i].Status = batchItemChecking
		})

		res := runBatchCheckItem(app, acc, platform, purchase, itemTimeout)

		var (
			status  string
			errText string
		)

		switch {
		case res.skipped:
			status = batchItemStalled
			errText = fmt.Sprintf("no response within %s; skipped", itemTimeout)
		case res.checkErr == nil:
			status = batchItemAvailable
		case errors.Is(res.checkErr, appstore.ErrLicenseRequired):
			status = batchItemLicenseRequired
		case res.purchaseErr != nil:
			status = batchItemError
			errText = fmt.Sprintf("failed to obtain license: %v", res.purchaseErr)
		default:
			status = batchItemError
			errText = res.checkErr.Error()
		}

		batchCheckJobs.update(job.ID, func(j *batchCheckJob) {
			item := &j.Items[i]
			item.Status = status
			item.Error = errText

			if status == batchItemAvailable {
				item.Version = res.out.Version
				item.ExternalVersionIdentifiers = res.out.ExternalVersionIdentifiers
				item.LatestExternalVersionID = res.out.LatestExternalVersionID
			}

			j.Done = i + 1
			j.Progress = float64(j.Done) / float64(j.Total) * 100.0
		})

		// Keep the refreshed account for the next apps, unless the probe was
		// skipped and its goroutine still owns the value.
		if !res.skipped {
			acc = res.acc
		}
	}

	batchCheckJobs.update(job.ID, func(j *batchCheckJob) {
		j.Status = "completed"
		j.Progress = 100.0
	})
}

func handleAPIBatchCheckStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	jobID := r.URL.Query().Get("jobId")
	if jobID == "" {
		jsonError(w, http.StatusBadRequest, "jobId is required")
		return
	}

	job, found := batchCheckJobs.get(jobID)
	if !found {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}

	jsonResponse(w, http.StatusOK, job)
}

// batchDownloadItem mirrors the live DownloadJob of one app in a batch while
// keeping the result available for aggregation.
type batchDownloadItem struct {
	JobID      string  `json:"jobId,omitempty"`
	AppID      int64   `json:"appId"`
	Name       string  `json:"name"`
	VersionID  string  `json:"versionId,omitempty"`
	Status     string  `json:"status"`
	Progress   float64 `json:"progress"`
	BytesRead  int64   `json:"bytesRead"`
	TotalBytes int64   `json:"totalBytes"`
	Error      string  `json:"error,omitempty"`
	Warning    string  `json:"warning,omitempty"`
	OutputPath string  `json:"outputPath,omitempty"`
	StartedAt  int64   `json:"startedAt,omitempty"`
	// StalledFor is how many seconds the item has been running without
	// receiving a single new byte. It is a live counter for the UI and is
	// reset to zero whenever progress happens.
	StalledFor float64 `json:"stalledFor,omitempty"`

	// abandoned is set when the watchdog gave up on the item while the
	// underlying download is still running in the background. It lets the
	// status endpoint report the real outcome if the download catches up.
	abandoned bool
}

// batchDownloadJob tracks a mass download of selected apps. Apps are processed
// sequentially through the same DownloadJob machinery as single downloads so
// both the batch progress and the existing job tracker stay consistent.
type batchDownloadJob struct {
	ID        string              `json:"id"`
	Status    string              `json:"status"` // "running", "completed"
	Total     int                 `json:"total"`
	Progress  float64             `json:"progress"` // 0-100, equal weight per app
	Items     []batchDownloadItem `json:"items"`
	Error     string              `json:"error,omitempty"`
	CreatedAt int64               `json:"createdAt"`
}

type batchDownloadTracker struct {
	sync.RWMutex
	jobs map[string]*batchDownloadJob
	// skips holds, per job ID, the indexes the user asked to skip. A skip can
	// arrive while the item is already running (the watchdog picks it up) or
	// while it is still queued (the loop notices it before starting it).
	skips map[string]map[int]bool
}

var batchDownloadJobs = &batchDownloadTracker{
	jobs:  make(map[string]*batchDownloadJob),
	skips: make(map[string]map[int]bool),
}

// requestSkip marks an item to be skipped. Index is resolved from the app ID
// so the API caller only needs what the UI already knows.
func (t *batchDownloadTracker) requestSkip(id string, appID int64) (int, bool) {
	t.Lock()
	defer t.Unlock()

	job, ok := t.jobs[id]
	if !ok {
		return 0, false
	}

	for i := range job.Items {
		if job.Items[i].AppID == appID {
			if t.skips[id] == nil {
				t.skips[id] = make(map[int]bool)
			}

			t.skips[id][i] = true

			return i, true
		}
	}

	return 0, false
}

// isSkipped reports whether the user asked to skip an item.
func (t *batchDownloadTracker) isSkipped(id string, index int) bool {
	t.RLock()
	defer t.RUnlock()

	return t.skips[id][index]
}

func (t *batchDownloadTracker) add(job *batchDownloadJob) {
	t.Lock()
	defer t.Unlock()
	t.jobs[job.ID] = job
}

func (t *batchDownloadTracker) update(id string, fn func(*batchDownloadJob)) {
	t.Lock()
	defer t.Unlock()
	if job, ok := t.jobs[id]; ok {
		fn(job)
	}
}

func (t *batchDownloadTracker) get(id string) (*batchDownloadJob, bool) {
	t.RLock()
	defer t.RUnlock()

	job, ok := t.jobs[id]
	if !ok {
		return nil, false
	}

	copyJob := *job
	copyJob.Items = make([]batchDownloadItem, len(job.Items))
	copy(copyJob.Items, job.Items)

	return &copyJob, true
}

type batchDownloadRequestPayload struct {
	Platform   string `json:"platform"`
	OutputPath string `json:"outputPath"`
	// Purchase is a pointer so an API client that omits the field keeps the
	// previous default behaviour (auto-acquiring licenses).
	Purchase *bool `json:"purchase"`
	// StallTimeoutSec and ItemTimeoutSec are optional watchdog settings. Zero
	// or a missing field keeps the built-in defaults.
	StallTimeoutSec int `json:"stallTimeoutSec"`
	ItemTimeoutSec  int `json:"itemTimeoutSec"`
	Items           []struct {
		AppID     int64  `json:"appId"`
		Name      string `json:"name"`
		VersionID string `json:"externalVersionId"`
	} `json:"items"`
}

func handleAPIBatchDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req batchDownloadRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if len(req.Items) == 0 {
		jsonError(w, http.StatusBadRequest, "at least one app is required")
		return
	}

	for _, item := range req.Items {
		if item.AppID == 0 {
			jsonError(w, http.StatusBadRequest, "app ID is required")
			return
		}
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "you must log in to download apps")
		return
	}

	job := &batchDownloadJob{
		ID:        fmt.Sprintf("batch_download_%d", time.Now().UnixNano()),
		Status:    "running",
		Total:     len(req.Items),
		Items:     make([]batchDownloadItem, len(req.Items)),
		CreatedAt: time.Now().Unix(),
	}

	for i, item := range req.Items {
		job.Items[i] = batchDownloadItem{
			AppID:     item.AppID,
			Name:      item.Name,
			VersionID: item.VersionID,
		}
	}

	batchDownloadJobs.add(job)

	go executeBatchDownloadJob(job, req, info.Account)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"batchId": job.ID,
		"total":   job.Total,
	})
}

// skippedByUserMessage is stored as the error text of an item the user asked
// to skip, so the reason is visible in the batch row.
const skippedByUserMessage = "skipped by user"

// batchWatchdogOutcome is the verdict of the per-item watchdog. A non-nil
// outcome means the batch stopped waiting on the item and moved on; the
// underlying download may still be running in the background.
type batchWatchdogOutcome struct {
	status string
	reason string
}

func executeBatchDownloadJob(job *batchDownloadJob, req batchDownloadRequestPayload, acc appstore.Account) {
	stallTimeout := timeoutFromSeconds(req.StallTimeoutSec, defaultStallTimeout)
	itemTimeout := timeoutFromSeconds(req.ItemTimeoutSec, defaultItemTimeout)

	// Keep a private snapshot of the immutable inputs; the job map itself is
	// only mutated through the tracker lock.
	items := make([]batchDownloadItem, len(job.Items))
	copy(items, job.Items)

	for i := range items {
		// An item can already be marked as skipped while it is still queued.
		if batchDownloadJobs.isSkipped(job.ID, i) {
			batchDownloadJobs.update(job.ID, func(j *batchDownloadJob) {
				j.Items[i].Status = batchItemSkipped
				j.Items[i].Error = skippedByUserMessage
				j.Items[i].Progress = 100.0
			})

			continue
		}

		acc = runBatchDownloadItem(job, i, items[i], req, acc, stallTimeout, itemTimeout)
	}

	batchDownloadJobs.update(job.ID, func(j *batchDownloadJob) {
		j.Status = "completed"
	})
}

// runBatchDownloadItem downloads a single app while watching it.
//
// The download itself cannot be cancelled: the App Store calls block and the
// HTTP client deliberately has no overall timeout so multi-gigabyte packages
// can stream. Instead the item runs in its own goroutine and a watchdog
// decides when to stop waiting on it — because the user asked to skip it,
// because no new byte arrived for stallTimeout, or because the item exceeded
// itemTimeout. The batch then carries on with the next app. An abandoned
// download keeps running in the background and the status endpoint reconciles
// it if it eventually finishes.
func runBatchDownloadItem(
	job *batchDownloadJob,
	index int,
	item batchDownloadItem,
	req batchDownloadRequestPayload,
	acc appstore.Account,
	stallTimeout, itemTimeout time.Duration,
) appstore.Account {
	downloadJob := &DownloadJob{
		ID:         fmt.Sprintf("job_%d", time.Now().UnixNano()),
		AppID:      item.AppID,
		AppName:    item.Name,
		Status:     "queued",
		CreatedAt:  time.Now().Unix(),
		OutputPath: req.OutputPath,
	}
	activeJobs.set(downloadJob)

	started := time.Now()

	batchDownloadJobs.update(job.ID, func(j *batchDownloadJob) {
		it := &j.Items[index]
		it.JobID = downloadJob.ID
		it.Status = downloadJob.Status
		it.StartedAt = started.Unix()
	})

	done := make(chan appstore.Account, 1)

	go func() {
		done <- executeDownloadJob(downloadJob, downloadRequestPayload{
			AppID:             item.AppID,
			OutputPath:        req.OutputPath,
			ExternalVersionID: item.VersionID,
			Platform:          req.Platform,
			Purchase:          batchAutoPurchase(req.Purchase),
		}, acc)
	}()

	ticker := time.NewTicker(watchdogTick)
	defer ticker.Stop()

	var (
		finished   bool
		lastBytes  = int64(-1)
		lastChange = started
		outcome    *batchWatchdogOutcome
	)

	for !finished {
		select {
		case next := <-done:
			acc = next
			finished = true

		case <-ticker.C:
			now := time.Now()

			live, ok := activeJobs.get(downloadJob.ID)
			if ok && live.BytesRead != lastBytes {
				lastBytes = live.BytesRead
				lastChange = now
			}

			stalledFor := now.Sub(lastChange)

			batchDownloadJobs.update(job.ID, func(j *batchDownloadJob) {
				j.Items[index].StalledFor = stalledFor.Seconds()
			})

			switch {
			case batchDownloadJobs.isSkipped(job.ID, index):
				outcome = &batchWatchdogOutcome{status: batchItemSkipped, reason: skippedByUserMessage}
			case ok && live.Status == "downloading" && stalledFor >= stallTimeout:
				// No byte moved for the whole stall window: treat it as hung.
				outcome = &batchWatchdogOutcome{
					status: batchItemStalled,
					reason: fmt.Sprintf("no download progress for %s; skipped, the app may still finish in the background", stallTimeout),
				}
			case now.Sub(started) >= itemTimeout:
				// Covers the phases without measurable progress (license,
				// patching) as well as a permanently slow download.
				outcome = &batchWatchdogOutcome{
					status: batchItemStalled,
					reason: fmt.Sprintf("exceeded the %s limit for a single app; skipped", itemTimeout),
				}
			default:
				continue
			}

			finished = true
		}
	}

	batchDownloadJobs.update(job.ID, func(j *batchDownloadJob) {
		it := &j.Items[index]
		it.StalledFor = 0

		if outcome != nil {
			// The abandoned goroutine still owns downloadJob, so only the
			// batch item is updated here.
			it.Status = outcome.status
			it.Error = outcome.reason
			it.Progress = 100.0
			it.abandoned = true

			return
		}

		it.Status = downloadJob.Status
		it.Error = downloadJob.Error
		it.Warning = downloadJob.Warning
		it.OutputPath = downloadJob.OutputPath
		it.BytesRead = downloadJob.BytesRead
		it.TotalBytes = downloadJob.TotalBytes
	})

	if outcome != nil {
		// Stop showing the item as active in the Downloads tab. The abandoned
		// goroutine may still overwrite this if it finishes later, which is
		// exactly what the batch status endpoint reconciles.
		activeJobs.update(downloadJob.ID, func(j *DownloadJob) {
			j.Status = outcome.status
			j.Error = outcome.reason
		})
	}

	return acc
}

func handleAPIBatchDownloadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	batchID := r.URL.Query().Get("batchId")
	if batchID == "" {
		jsonError(w, http.StatusBadRequest, "batchId is required")
		return
	}

	job, found := batchDownloadJobs.get(batchID)
	if !found {
		jsonError(w, http.StatusNotFound, "batch not found")
		return
	}

	// Merge the live per-job state (bytes progress, status) into the returned
	// copy so the frontend can show one overall percent with equal weight per
	// app plus the usual per-job details. The stored job is never mutated here.
	totalProgress := 0.0
	errorsCount := 0
	skippedCount := 0

	for i := range job.Items {
		item := &job.Items[i]

		if item.JobID != "" {
			if live, ok := activeJobs.get(item.JobID); ok {
				if item.abandoned {
					// The skipped download still runs in the background, so
					// only a late success overrides the watchdog's verdict.
					if live.Status == "completed" {
						item.Status = "completed"
						item.Progress = 100.0
						item.Error = ""
						item.Warning = "finished in the background after being skipped"
						item.OutputPath = live.OutputPath
						item.BytesRead = live.BytesRead
						item.TotalBytes = live.TotalBytes
					}
				} else {
					item.Status = live.Status
					item.Progress = live.Progress
					item.Error = live.Error
					item.Warning = live.Warning
					item.OutputPath = live.OutputPath
					item.BytesRead = live.BytesRead
					item.TotalBytes = live.TotalBytes
				}
			}
		}

		switch item.Status {
		case "completed":
			item.Progress = 100.0
		case "error":
			item.Progress = 100.0
			errorsCount++
		case batchItemStalled, batchItemSkipped:
			item.Progress = 100.0
			skippedCount++
		}

		totalProgress += item.Progress
	}

	job.Progress = 0.0
	if job.Total > 0 {
		job.Progress = totalProgress / float64(job.Total)
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":             job.ID,
		"status":         job.Status,
		"total":          job.Total,
		"progress":       job.Progress,
		"errors":         errorsCount,
		"skipped":        skippedCount,
		"items":          job.Items,
		"createdAt":      job.CreatedAt,
		"completedCount": completedCount(job.Items),
	})
}

// handleAPIBatchDownloadSkip lets the user give up on a single app of a
// running batch so the batch can move on to the next one. It is the manual
// counterpart of the watchdog: use it when an app is visibly stuck but has not
// hit the automatic timeouts yet.
func handleAPIBatchDownloadSkip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BatchID string `json:"batchId"`
		AppID   int64  `json:"appId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if req.BatchID == "" || req.AppID == 0 {
		jsonError(w, http.StatusBadRequest, "batchId and appId are required")
		return
	}

	index, ok := batchDownloadJobs.requestSkip(req.BatchID, req.AppID)
	if !ok {
		jsonError(w, http.StatusNotFound, "batch or app not found")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"index":   index,
	})
}

// completedCount reports how many items are finished, including the ones that
// ended in an error or were skipped, so the UI can show "N of M processed".
func completedCount(items []batchDownloadItem) int {
	count := 0

	for _, item := range items {
		switch item.Status {
		case "completed", "error", batchItemStalled, batchItemSkipped:
			count++
		}
	}

	return count
}
