package cmd

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/majd/ipatool/v2/pkg/anisette"
	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/spf13/cobra"
)

//go:embed gui_assets/*
var embeddedAssets embed.FS

// DownloadJob represents an asynchronous download task tracked by the GUI.
type DownloadJob struct {
	ID         string  `json:"id"`
	AppName    string  `json:"appName"`
	BundleID   string  `json:"bundleId"`
	AppID      int64   `json:"appId"`
	Version    string  `json:"version"`
	Progress   float64 `json:"progress"`
	BytesRead  int64   `json:"bytesRead"`
	TotalBytes int64   `json:"totalBytes"`
	Speed      string  `json:"speed"`
	Status     string  `json:"status"` // "queued", "purchasing", "downloading", "patching", "completed", "error"
	Error      string  `json:"error,omitempty"`
	OutputPath string  `json:"outputPath,omitempty"`
	CreatedAt  int64   `json:"createdAt"`
}

type jobTracker struct {
	sync.RWMutex
	jobs map[string]*DownloadJob
}

var activeJobs = &jobTracker{
	jobs: make(map[string]*DownloadJob),
}

// versionMetadataCache caches per-build display version and release date so
// revisiting the version-history table does not re-read each IPA from Apple.
type versionMetadataCache struct {
	sync.RWMutex
	m map[string]map[string]versionMetaEntry // appID -> versionID -> entry
}

type versionMetaEntry struct {
	DisplayVersion   string
	ReleaseDate      string
	MinimumOSVersion string
}

var versionMetaCache = &versionMetadataCache{
	m: make(map[string]map[string]versionMetaEntry),
}

func (jt *jobTracker) get(id string) (*DownloadJob, bool) {
	jt.RLock()
	defer jt.RUnlock()
	j, exists := jt.jobs[id]
	if !exists {
		return nil, false
	}
	// return copy
	copyJob := *j
	return &copyJob, true
}

func (jt *jobTracker) set(j *DownloadJob) {
	jt.Lock()
	defer jt.Unlock()
	jt.jobs[j.ID] = j
}

// progressWriter implements io.Writer to track bytes written during download.
type progressWriter struct {
	jobID        string
	totalBytes   int64
	bytesRead    int64
	startTime    time.Time
	lastUpdate   time.Time
	lastBytes    int64
	currentSpeed string
	mu           sync.Mutex
}

// setTotal records the total package size once it becomes known so the
// progress percentage and weight can be computed.
func (pw *progressWriter) setTotal(total int64) {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if total > 0 {
		pw.totalBytes = total
		if activeJobs != nil {
			activeJobs.Lock()
			if j, ok := activeJobs.jobs[pw.jobID]; ok {
				j.TotalBytes = total
			}
			activeJobs.Unlock()
		}
	}
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.mu.Lock()
	pw.bytesRead += int64(n)

	now := time.Now()
	if now.Sub(pw.lastUpdate) >= 300*time.Millisecond {
		diffBytes := pw.bytesRead - pw.lastBytes
		duration := now.Sub(pw.lastUpdate).Seconds()
		if duration > 0 {
			speedBps := float64(diffBytes) / duration
			if speedBps > 1024*1024 {
				pw.currentSpeed = fmt.Sprintf("%.2f MB/s", speedBps/(1024*1024))
			} else if speedBps > 1024 {
				pw.currentSpeed = fmt.Sprintf("%.1f KB/s", speedBps/1024)
			} else {
				pw.currentSpeed = fmt.Sprintf("%d B/s", int64(speedBps))
			}
		}
		pw.lastUpdate = now
		pw.lastBytes = pw.bytesRead

		if activeJobs != nil {
			activeJobs.Lock()
			if j, ok := activeJobs.jobs[pw.jobID]; ok {
				j.BytesRead = pw.bytesRead
				if pw.totalBytes > 0 {
					j.TotalBytes = pw.totalBytes
					j.Progress = (float64(pw.bytesRead) / float64(pw.totalBytes)) * 100.0
				}
				j.Speed = pw.currentSpeed
				j.Status = "downloading"
			}
			activeJobs.Unlock()
		}
	}
	pw.mu.Unlock()

	return n, nil
}

func guiCmd() *cobra.Command {
	var (
		host      string
		port      int
		noBrowser bool
	)

	cmd := &cobra.Command{
		Use:   "gui",
		Short: "Launch the graphical user interface for ipatool",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGUIServer(host, port, noBrowser)
		},
	}

	cmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host address to bind GUI server to")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to run GUI server on (0 for random available port)")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Do not automatically open default web browser on start")

	return cmd
}

func runGUIServer(host string, port int, noBrowser bool) error {
	// Setup asset filesystem
	assetsSub, err := fs.Sub(embeddedAssets, "gui_assets")
	if err != nil {
		return fmt.Errorf("failed to open embedded assets: %w", err)
	}

	mux := http.NewServeMux()

	// Static assets handler
	assetsFS := http.FS(assetsSub)
	fileServer := http.FileServer(assetsFS)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			data, readErr := fs.ReadFile(assetsSub, "index.html")
			if readErr != nil {
				http.Error(w, "index.html not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(data)
			return
		}

		if r.URL.Path == "/assets/style.css" {
			data, readErr := fs.ReadFile(assetsSub, "style.css")
			if readErr != nil {
				http.Error(w, "style.css not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = w.Write(data)
			return
		}

		if r.URL.Path == "/assets/app.js" {
			data, readErr := fs.ReadFile(assetsSub, "app.js")
			if readErr != nil {
				http.Error(w, "app.js not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			_, _ = w.Write(data)
			return
		}

		fileServer.ServeHTTP(w, r)
	})

	// API Handlers
	mux.HandleFunc("/api/status", handleAPIStatus)
	mux.HandleFunc("/api/auth/login", handleAPILogin)
	mux.HandleFunc("/api/auth/login/mzfinance", handleAPILoginMZFinance)
	mux.HandleFunc("/api/auth/revoke", handleAPIRevoke)
	mux.HandleFunc("/api/auth/export", handleAPIExport)
	mux.HandleFunc("/api/auth/import", handleAPIImport)
	mux.HandleFunc("/api/search", handleAPISearch)
	mux.HandleFunc("/api/purchase", handleAPIPurchase)
	mux.HandleFunc("/api/download", handleAPIDownload)
	mux.HandleFunc("/api/download/status", handleAPIDownloadStatus)
	mux.HandleFunc("/api/icloud/status", handleAPIICloudStatus)
	mux.HandleFunc("/api/downloads/active", handleAPIActiveDownloads)
	mux.HandleFunc("/api/versions", handleAPIVersions)
	mux.HandleFunc("/api/version-metadata", handleAPIVersionMetadata)
	mux.HandleFunc("/api/batch/check", handleAPIBatchCheck)
	mux.HandleFunc("/api/batch/check/status", handleAPIBatchCheckStatus)
	mux.HandleFunc("/api/batch/download", handleAPIBatchDownload)
	mux.HandleFunc("/api/batch/download/status", handleAPIBatchDownloadStatus)
	mux.HandleFunc("/api/install/devices", handleAPIInstallDevices)
	mux.HandleFunc("/api/install/upload", handleAPIInstallUpload)
	mux.HandleFunc("/api/install/status", handleAPIInstallStatus)
	mux.HandleFunc("/api/open-folder", handleAPIOpenFolder)

	// Wrap mux with CORS headers for local/preview access
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("failed to bind server to %s:%d: %w", host, port, err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port

	serverURL := fmt.Sprintf("http://localhost:%d", actualPort)
	if host != "127.0.0.1" && host != "localhost" {
		serverURL = fmt.Sprintf("http://%s:%d", host, actualPort)
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("   ipatool GUI is running!\n")
	fmt.Printf("   URL: %s\n", serverURL)
	fmt.Printf("   Press Ctrl+C to stop the GUI server.\n")
	fmt.Printf("=======================================================\n\n")

	if !noBrowser {
		go openBrowser(serverURL)
	}

	server := &http.Server{
		Handler: handler,
	}

	return server.Serve(listener)
}

func openBrowser(url string) {
	time.Sleep(300 * time.Millisecond)

	switch runtime.GOOS {
	case "windows":
		// On Windows, try Edge App Mode first for a sleek desktop app window!
		edgePath := os.ExpandEnv(`%ProgramFiles(x86)%\Microsoft\Edge\Application\msedge.exe`)
		if _, err := os.Stat(edgePath); err == nil {
			_ = exec.Command(edgePath, fmt.Sprintf("--app=%s", url)).Start()
			return
		}
		edgePath64 := os.ExpandEnv(`%ProgramFiles%\Microsoft\Edge\Application\msedge.exe`)
		if _, err := os.Stat(edgePath64); err == nil {
			_ = exec.Command(edgePath64, fmt.Sprintf("--app=%s", url)).Start()
			return
		}
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

// API JSON Helpers
func jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, statusCode int, message string) {
	jsonResponse(w, statusCode, map[string]interface{}{
		"success": false,
		"message": message,
	})
}

// ==========================================
// API Handlers
// ==========================================

func handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"authenticated": false,
			"account":       nil,
			"version":       version,
			"os":            runtime.GOOS,
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"account": map[string]interface{}{
			"name":       info.Account.Name,
			"email":      info.Account.Email,
			"storefront": info.Account.StoreFront,
			"dsid":       info.Account.DirectoryServicesID,
		},
		"version": version,
		"os":      runtime.GOOS,
	})
}

type loginRequestPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	AuthCode string `json:"authCode"`
}

func handleAPILogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req loginRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	if req.Email == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	bag, err := dependencies.AppStore.Bag(appstore.BagInput{})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get Apple bag: %v", err))
		return
	}

	output, err := dependencies.AppStore.Login(appstore.LoginInput{
		Email:    req.Email,
		Password: req.Password,
		AuthCode: req.AuthCode,
		Endpoint: bag.AuthEndpoint,
	})

	if err != nil {
		if errors.Is(err, appstore.ErrAuthCodeRequired) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"success":          false,
				"authCodeRequired": true,
				"message":          "2FA verification code is required",
			})
			return
		}

		// On Windows, the GSA login flow depends on a locally installed and
		// signed-in iCloud (Microsoft Store) to produce the anisette headers.
		// If that step failed, tell the user exactly how to fix it rather than
		// showing a raw anisette error.
		if runtime.GOOS == "windows" && strings.Contains(err.Error(), "anisette") {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"success":          false,
				"anisetteRequired": true,
				"message":          err.Error(),
			})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"account": map[string]interface{}{
			"name":       output.Account.Name,
			"email":      output.Account.Email,
			"storefront": output.Account.StoreFront,
			"dsid":       output.Account.DirectoryServicesID,
		},
	})
}

// handleAPILoginMZFinance is the diagnostic login endpoint used by the
// macOS-only "Войти в Apple ID ТЕСТ" button. It runs the stable legacy
// MZFinance authenticate flow (GSA handshake first, then MZFinance directly),
// bypassing the glitchy native/fast path, without changing the standard
// /api/auth/login behaviour.
func handleAPILoginMZFinance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req loginRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	if req.Email == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	output, err := dependencies.AppStore.LoginMZFinance(appstore.LoginInput{
		Email:    req.Email,
		Password: req.Password,
		AuthCode: req.AuthCode,
	})

	if err != nil {
		if errors.Is(err, appstore.ErrAuthCodeRequired) {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"success":          false,
				"authCodeRequired": true,
				"message":          "2FA verification code is required",
			})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"account": map[string]interface{}{
			"name":       output.Account.Name,
			"email":      output.Account.Email,
			"storefront": output.Account.StoreFront,
			"dsid":       output.Account.DirectoryServicesID,
		},
	})
}

func handleAPIRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	err := dependencies.AppStore.Revoke()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func handleAPIExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	acc := info.Account
	acc.Password = ""

	w.Header().Set("Content-Disposition", "attachment; filename=account-session.json")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(acc)
}

func handleAPIImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var acc appstore.Account
	if err := json.NewDecoder(r.Body).Decode(&acc); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid account session JSON")
		return
	}

	out, err := dependencies.AppStore.ImportAccount(appstore.ImportAccountInput{
		Account: acc,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"email":   out.Account.Email,
	})
}

func handleAPISearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	term := r.URL.Query().Get("term")
	if term == "" {
		jsonError(w, http.StatusBadRequest, "search term is required")
		return
	}

	platformStr := r.URL.Query().Get("platform")
	platform, err := appstore.ParsePlatform(platformStr)
	if err != nil {
		platform = appstore.PlatformIPhone
	}

	limit := int64(25)
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, parseErr := strconv.ParseInt(l, 10, 64); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}

	infoResult, err := dependencies.AppStore.AccountInfo()
	var acc appstore.Account
	if err == nil {
		acc = infoResult.Account
	} else {
		// Fallback to default US storefront if not logged in
		acc = appstore.Account{StoreFront: "143441-1,29"}
	}

	out, err := dependencies.AppStore.Search(appstore.SearchInput{
		Account:  acc,
		Term:     term,
		Limit:    limit,
		Platform: platform,
	})

	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   out.Count,
		"results": out.Results,
	})
}

type purchaseRequestPayload struct {
	BundleID string `json:"bundleId"`
}

func handleAPIPurchase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req purchaseRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "you must log in to purchase apps")
		return
	}

	lookup, err := dependencies.AppStore.Lookup(appstore.LookupInput{
		Account:  info.Account,
		BundleID: req.BundleID,
	})
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("app lookup failed: %v", err))
		return
	}

	purchaseErr := dependencies.AppStore.Purchase(appstore.PurchaseInput{
		Account: info.Account,
		App:     lookup.App,
	})

	alreadyOwned := errors.Is(purchaseErr, appstore.ErrLicenseAlreadyExists)
	if purchaseErr != nil && !alreadyOwned {
		jsonError(w, http.StatusInternalServerError, purchaseErr.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"alreadyOwned": alreadyOwned,
	})
}

type downloadRequestPayload struct {
	BundleID          string `json:"bundleId"`
	AppID             int64  `json:"appId"`
	OutputPath        string `json:"outputPath"`
	ExternalVersionID string `json:"externalVersionId"`
	Platform          string `json:"platform"`
	Purchase          bool   `json:"purchase"`
}

func handleAPIDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req downloadRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	if req.AppID == 0 && req.BundleID == "" {
		jsonError(w, http.StatusBadRequest, "either app ID or bundle ID is required")
		return
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "you must log in to download apps")
		return
	}

	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	job := &DownloadJob{
		ID:         jobID,
		BundleID:   req.BundleID,
		AppID:      req.AppID,
		Status:     "queued",
		CreatedAt:  time.Now().Unix(),
		OutputPath: req.OutputPath,
	}
	activeJobs.set(job)

	go executeDownloadJob(job, req, info.Account)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"jobId":   jobID,
	})
}

func executeDownloadJob(job *DownloadJob, req downloadRequestPayload, acc appstore.Account) {
	platform, _ := appstore.ParsePlatform(req.Platform)
	app := appstore.App{ID: req.AppID}

	if req.BundleID != "" {
		lookupResult, err := dependencies.AppStore.Lookup(appstore.LookupInput{
			Account:  acc,
			BundleID: req.BundleID,
			Platform: platform,
		})
		if err == nil {
			app = lookupResult.App
			job.AppName = app.Name
		}
	}

	// Step 1: Auto purchase if requested
	if req.Purchase {
		job.Status = "purchasing"
		activeJobs.set(job)
		_ = dependencies.AppStore.Purchase(appstore.PurchaseInput{Account: acc, App: app})
	}

	// Step 2: Download
	job.Status = "downloading"
	activeJobs.set(job)

	pw := &progressWriter{
		jobID:      job.ID,
		startTime:  time.Now(),
		lastUpdate: time.Now(),
	}

	out, err := dependencies.AppStore.Download(appstore.DownloadInput{
		Account:           acc,
		App:               app,
		OutputPath:        req.OutputPath,
		ExternalVersionID: req.ExternalVersionID,
		Platform:          platform,
		ProgressWriter:    pw,
		OnTotalBytes:      pw.setTotal,
	})

	if err != nil {
		job.Status = "error"
		job.Error = err.Error()
		activeJobs.set(job)
		return
	}

	// Step 3: Replicate SINF signatures
	job.Status = "patching"
	activeJobs.set(job)

	err = dependencies.AppStore.ReplicateSinf(appstore.ReplicateSinfInput{
		Sinfs:       out.Sinfs,
		PackagePath: out.DestinationPath,
	})

	if err != nil {
		job.Status = "error"
		job.Error = fmt.Sprintf("failed to replicate signature: %v", err)
		activeJobs.set(job)
		return
	}

	// Step 4: Finished!
	job.Status = "completed"
	job.Progress = 100.0
	job.OutputPath = out.DestinationPath
	activeJobs.set(job)
}

func handleAPIDownloadStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("jobId")
	if jobID == "" {
		jsonError(w, http.StatusBadRequest, "jobId is required")
		return
	}

	job, found := activeJobs.get(jobID)
	if !found {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}

	jsonResponse(w, http.StatusOK, job)
}

// handleAPIActiveDownloads returns every non-terminal download job (single or
// batch), newest first, so the Downloads tab can render real active jobs even
// after a page reload or for jobs started from other tabs.
func handleAPIActiveDownloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	activeJobs.RLock()
	jobs := make([]*DownloadJob, 0, len(activeJobs.jobs))
	for _, job := range activeJobs.jobs {
		if job.Status == "queued" || job.Status == "purchasing" || job.Status == "downloading" || job.Status == "patching" {
			copyJob := *job
			jobs = append(jobs, &copyJob)
		}
	}
	activeJobs.RUnlock()

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt > jobs[j].CreatedAt
	})

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"jobs":    jobs,
	})
}

func handleAPIICloudStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	status := anisette.CheckICloud()
	jsonResponse(w, http.StatusOK, status)
}

func handleAPIVersions(w http.ResponseWriter, r *http.Request) {
	bundleID := r.URL.Query().Get("bundleId")
	appIDStr := r.URL.Query().Get("appId")
	var appID int64
	if appIDStr != "" {
		appID, _ = strconv.ParseInt(appIDStr, 10, 64)
	}

	if bundleID == "" && appID == 0 {
		jsonError(w, http.StatusBadRequest, "bundleId or appId is required")
		return
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "you must log in to list versions")
		return
	}

	app := appstore.App{ID: appID}
	if bundleID != "" {
		lookupResult, err := dependencies.AppStore.Lookup(appstore.LookupInput{Account: info.Account, BundleID: bundleID})
		if err != nil {
			jsonError(w, http.StatusBadRequest, fmt.Sprintf("lookup failed: %v", err))
			return
		}
		app = lookupResult.App
	}

	out, err := dependencies.AppStore.ListVersions(appstore.ListVersionsInput{
		Account: info.Account,
		App:     app,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":                    true,
		"name":                       app.Name,
		"bundleID":                   app.BundleID,
		"externalVersionIdentifiers": out.ExternalVersionIdentifiers,
	})
}

func handleAPIVersionMetadata(w http.ResponseWriter, r *http.Request) {
	bundleID := r.URL.Query().Get("bundleId")
	appIDStr := r.URL.Query().Get("appId")
	versionID := r.URL.Query().Get("versionId")

	var appID int64
	if appIDStr != "" {
		appID, _ = strconv.ParseInt(appIDStr, 10, 64)
	}

	if versionID == "" {
		jsonError(w, http.StatusBadRequest, "versionId is required")
		return
	}

	// Serve from cache when available (keyed by appID:versionID).
	if appID != 0 {
		versionMetaCache.RLock()
		entry, ok := versionMetaCache.m[strconv.FormatInt(appID, 10)][versionID]
		versionMetaCache.RUnlock()
		if ok {
			jsonResponse(w, http.StatusOK, map[string]interface{}{
				"success":          true,
				"displayVersion":   entry.DisplayVersion,
				"releaseDate":      entry.ReleaseDate,
				"minimumOSVersion": entry.MinimumOSVersion,
			})
			return
		}
	}

	info, err := dependencies.AppStore.AccountInfo()
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	app := appstore.App{ID: appID}
	// Only resolve the bundle ID into an app ID when the caller did not already
	// supply an app ID; this avoids an extra lookup for every build.
	if bundleID != "" && appID == 0 {
		lookupResult, err := dependencies.AppStore.Lookup(appstore.LookupInput{Account: info.Account, BundleID: bundleID})
		if err == nil {
			app = lookupResult.App
			appID = app.ID
		}
	}

	out, err := dependencies.AppStore.GetVersionMetadata(appstore.GetVersionMetadataInput{
		Account:   info.Account,
		App:       app,
		VersionID: versionID,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	releaseDate := out.ReleaseDate.Format("2006-01-02")

	if appID != 0 {
		versionMetaCache.Lock()
		if versionMetaCache.m[appIDKey(appID)] == nil {
			versionMetaCache.m[appIDKey(appID)] = map[string]versionMetaEntry{}
		}
		versionMetaCache.m[appIDKey(appID)][versionID] = versionMetaEntry{
			DisplayVersion:   out.DisplayVersion,
			ReleaseDate:      releaseDate,
			MinimumOSVersion: out.MinimumOSVersion,
		}
		versionMetaCache.Unlock()
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":          true,
		"displayVersion":   out.DisplayVersion,
		"releaseDate":      releaseDate,
		"minimumOSVersion": out.MinimumOSVersion,
	})
}

// appIDKey converts an app ID to its cache-map key.
func appIDKey(id int64) string {
	return strconv.FormatInt(id, 10)
}

type openFolderPayload struct {
	Path string `json:"path"`
}

func handleAPIOpenFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req openFolderPayload
	_ = json.NewDecoder(r.Body).Decode(&req)

	targetPath := req.Path
	if targetPath == "" {
		targetPath, _ = os.Getwd()
	} else {
		// If path points to a file, get parent dir
		fi, err := os.Stat(targetPath)
		if err == nil && !fi.IsDir() {
			targetPath = filepath.Dir(targetPath)
		}
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", targetPath)
	case "darwin":
		cmd = exec.Command("open", targetPath)
	default:
		cmd = exec.Command("xdg-open", targetPath)
	}

	if err := cmd.Start(); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open folder: %v", err))
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}
