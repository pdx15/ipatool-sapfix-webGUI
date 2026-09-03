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
)

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
	Items    []struct {
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

	go executeBatchCheckJob(job, req.Platform, info.Account, batchAutoPurchase(req.Purchase))

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"jobId":   job.ID,
		"total":   job.Total,
	})
}

// executeBatchCheckJob runs every app through the same sequence the Direct
// Download path uses (license acquisition attempt followed by the
// volumeStoreDownloadProduct request, minus the package transfer), filtering
// out apps for which the account has no license. Each finished app advances
// the overall progress by an equal step, so 100 apps mean 1% per pass.
func executeBatchCheckJob(job *batchCheckJob, platformStr string, acc appstore.Account, purchase bool) {
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

		// When auto-purchase is enabled, attempt to acquire the license (with
		// automatic session refresh when needed) before the direct-download
		// request. The last known account is reused even when the purchase call
		// itself fails, so a refreshed session is still available for the check.
		var purchaseErr error
		if purchase {
			var refreshed appstore.Account
			refreshed, _, purchaseErr = purchaseWithRetry(acc, app)
			if purchaseErr == nil {
				acc = refreshed
			}
		}

		_, out, checkErr := checkDownloadWithRetry(acc, app, platform)

		batchCheckJobs.update(job.ID, func(j *batchCheckJob) {
			item := &j.Items[i]
			switch {
			case checkErr == nil:
				item.Status = batchItemAvailable
				item.Version = out.Version
				item.ExternalVersionIdentifiers = out.ExternalVersionIdentifiers
				item.LatestExternalVersionID = out.LatestExternalVersionID
			case errors.Is(checkErr, appstore.ErrLicenseRequired):
				item.Status = batchItemLicenseRequired
			case purchaseErr != nil:
				item.Status = batchItemError
				item.Error = fmt.Sprintf("failed to obtain license: %v", purchaseErr)
			default:
				item.Status = batchItemError
				item.Error = checkErr.Error()
			}

			j.Done = i + 1
			j.Progress = float64(j.Done) / float64(j.Total) * 100.0
		})
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
}

var batchDownloadJobs = &batchDownloadTracker{
	jobs: make(map[string]*batchDownloadJob),
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
	Items    []struct {
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

func executeBatchDownloadJob(job *batchDownloadJob, req batchDownloadRequestPayload, acc appstore.Account) {
	// Keep a private snapshot of the immutable inputs; the job map itself is
	// only mutated through the tracker lock.
	items := make([]batchDownloadItem, len(job.Items))
	copy(items, job.Items)

	for i := range items {
		downloadJob := &DownloadJob{
			ID:         fmt.Sprintf("job_%d", time.Now().UnixNano()),
			AppID:      items[i].AppID,
			AppName:    items[i].Name,
			Status:     "queued",
			CreatedAt:  time.Now().Unix(),
			OutputPath: req.OutputPath,
		}
		activeJobs.set(downloadJob)

		batchDownloadJobs.update(job.ID, func(j *batchDownloadJob) {
			item := &j.Items[i]
			item.JobID = downloadJob.ID
			item.Status = downloadJob.Status
		})

		acc = executeDownloadJob(downloadJob, downloadRequestPayload{
			AppID:             items[i].AppID,
			OutputPath:        req.OutputPath,
			ExternalVersionID: req.Items[i].VersionID,
			Platform:          req.Platform,
			Purchase:          batchAutoPurchase(req.Purchase),
		}, acc)

		batchDownloadJobs.update(job.ID, func(j *batchDownloadJob) {
			item := &j.Items[i]
			item.Status = downloadJob.Status
			item.Error = downloadJob.Error
			item.Warning = downloadJob.Warning
			item.OutputPath = downloadJob.OutputPath
			item.BytesRead = downloadJob.BytesRead
			item.TotalBytes = downloadJob.TotalBytes
		})
	}

	batchDownloadJobs.update(job.ID, func(j *batchDownloadJob) {
		j.Status = "completed"
	})
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
	for i := range job.Items {
		item := &job.Items[i]
		if item.JobID != "" {
			if live, ok := activeJobs.get(item.JobID); ok {
				item.Status = live.Status
				item.Progress = live.Progress
				item.Error = live.Error
				item.Warning = live.Warning
				item.OutputPath = live.OutputPath
				item.BytesRead = live.BytesRead
				item.TotalBytes = live.TotalBytes
			}
		}

		if item.Status == "completed" {
			item.Progress = 100.0
		} else if item.Status == "error" {
			item.Progress = 100.0
			errorsCount++
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
		"items":          job.Items,
		"createdAt":      job.CreatedAt,
		"completedCount": completedCount(job.Items),
	})
}

func completedCount(items []batchDownloadItem) int {
	count := 0
	for _, item := range items {
		if item.Status == "completed" || item.Status == "error" {
			count++
		}
	}
	return count
}
