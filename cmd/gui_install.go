package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// install device info returned by /api/install/devices.
type installDevice struct {
	UDID           string `json:"udid"`
	Name           string `json:"name,omitempty"`
	ProductType    string `json:"productType,omitempty"`
	ProductVersion string `json:"productVersion,omitempty"`
	SerialNumber   string `json:"serialNumber,omitempty"`
}

// installTool describes one command required for device listing/installation.
type installTool struct {
	Name  string `json:"name"`
	Path  string `json:"path,omitempty"`
	Found bool   `json:"found"`
	Kind  string `json:"kind,omitempty"` // "list", "info" or "install"
}

// installJob tracks an asynchronous .ipa installation on a connected device.
type installJob struct {
	ID         string  `json:"id"`
	UDID       string  `json:"udid"`
	DeviceName string  `json:"deviceName,omitempty"`
	FileName   string  `json:"fileName,omitempty"`
	FilePath   string  `json:"-"`
	Status     string  `json:"status"` // "queued", "installing", "completed", "error"
	Progress   float64 `json:"progress"`
	Message    string  `json:"message,omitempty"`
	Log        string  `json:"log,omitempty"`
	Error      string  `json:"error,omitempty"`
	CreatedAt  int64   `json:"createdAt"`
}

type installJobTracker struct {
	sync.RWMutex
	jobs map[string]*installJob
}

var installJobs = &installJobTracker{
	jobs: make(map[string]*installJob),
}

func (t *installJobTracker) add(job *installJob) {
	t.Lock()
	defer t.Unlock()
	t.jobs[job.ID] = job

	// Keep the in-memory list bounded so long-running sessions do not grow
	// forever.
	const maxJobs = 100
	if len(t.jobs) <= maxJobs {
		return
	}
	ids := make([]string, 0, len(t.jobs))
	for id, job := range t.jobs {
		if job.Status != "queued" && job.Status != "installing" {
			ids = append(ids, id)
		}
	}
	// Keep the newest terminal jobs and always retain active ones.
	keep := len(t.jobs) - maxJobs
	for i := 0; i < len(ids) && keep > 0; i++ {
		delete(t.jobs, ids[i])
		keep--
	}
}

func (t *installJobTracker) get(id string) (*installJob, bool) {
	t.RLock()
	defer t.RUnlock()

	job, ok := t.jobs[id]
	if !ok {
		return nil, false
	}
	copyJob := *job
	return &copyJob, true
}

func (t *installJobTracker) update(id string, fn func(*installJob)) {
	t.Lock()
	defer t.Unlock()
	if job, ok := t.jobs[id]; ok {
		fn(job)
	}
}

func findInstallTool() string {
	if path := os.Getenv("IPATOOL_IDEVICEINSTALLER"); path != "" {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}

	for _, name := range []string{"ideviceinstaller", "ideviceinstaller.exe"} {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			return path
		}
	}

	// Check common installation locations when the tool is not on PATH.
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{
			`C:\Program Files\idevice\ideviceinstaller.exe`,
			`C:\Program Files (x86)\idevice\ideviceinstaller.exe`,
			`C:\Program Files\libimobiledevice\ideviceinstaller.exe`,
			`C:\Program Files (x86)\libimobiledevice\ideviceinstaller.exe`,
		}
	} else {
		candidates = []string{
			"/opt/homebrew/bin/ideviceinstaller",
			"/usr/local/bin/ideviceinstaller",
			"/opt/local/bin/ideviceinstaller",
			"/usr/bin/ideviceinstaller",
		}
	}

	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}

	return ""
}

func findDeviceListTool() string {
	if path := os.Getenv("IPATOOL_IDEVICE_ID"); path != "" {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}

	for _, name := range []string{"idevice_id", "idevice_id.exe"} {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			return path
		}
	}

	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{
			`C:\Program Files\idevice\idevice_id.exe`,
			`C:\Program Files (x86)\idevice\idevice_id.exe`,
			`C:\Program Files\libimobiledevice\idevice_id.exe`,
			`C:\Program Files (x86)\libimobiledevice\idevice_id.exe`,
		}
	} else {
		candidates = []string{
			"/opt/homebrew/bin/idevice_id",
			"/usr/local/bin/idevice_id",
			"/opt/local/bin/idevice_id",
			"/usr/bin/idevice_id",
		}
	}

	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}

	return ""
}

func findDeviceInfoTool() string {
	if path := os.Getenv("IPATOOL_IDEVICEDEVICEINFO"); path != "" {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}

	for _, name := range []string{"idevicedeviceinfo", "idevicedeviceinfo.exe"} {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			return path
		}
	}

	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{
			`C:\Program Files\idevice\idevicedeviceinfo.exe`,
			`C:\Program Files (x86)\idevice\idevicedeviceinfo.exe`,
			`C:\Program Files\libimobiledevice\idevicedeviceinfo.exe`,
			`C:\Program Files (x86)\libimobiledevice\idevicedeviceinfo.exe`,
		}
	} else {
		candidates = []string{
			"/opt/homebrew/bin/idevicedeviceinfo",
			"/usr/local/bin/idevicedeviceinfo",
			"/opt/local/bin/idevicedeviceinfo",
			"/usr/bin/idevicedeviceinfo",
		}
	}

	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}

	return ""
}

func runInstallCommand(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

func listInstallDevices() ([]installDevice, string, string, string) {
	listTool := findDeviceListTool()
	infoTool := findDeviceInfoTool()
	installTool := findInstallTool()

	if listTool == "" {
		return nil, "", infoTool, installTool
	}

	raw, err := runInstallCommand(5*time.Second, listTool, "-l")
	if err != nil {
		// idevice_id prints its error to stderr; report it if there is any.
		return nil, raw, infoTool, installTool
	}

	var devices []installDevice
	for _, line := range strings.Split(raw, "\n") {
		udid := strings.TrimSpace(line)
		if udid == "" || strings.HasPrefix(udid, "ERROR:") {
			continue
		}

		device := installDevice{UDID: udid}
		if infoTool != "" {
			device.Name = installDeviceField(infoTool, udid, "DeviceName")
			device.ProductType = installDeviceField(infoTool, udid, "ProductType")
			device.ProductVersion = installDeviceField(infoTool, udid, "ProductVersion")
			device.SerialNumber = installDeviceField(infoTool, udid, "SerialNumber")
		}
		devices = append(devices, device)
	}

	return devices, "", infoTool, installTool
}

func installDeviceField(tool, udid, key string) string {
	value, _ := runInstallCommand(3*time.Second, tool, "-u", udid, "-k", key)
	return strings.TrimSpace(value)
}

func handleAPIInstallDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	devices, listErr, infoTool, installTool := listInstallDevices()

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"devices": devices,
		"tools": []installTool{
			{Name: "idevice_id", Path: findDeviceListTool(), Found: findDeviceListTool() != "", Kind: "list"},
			{Name: "idevicedeviceinfo", Path: findDeviceInfoTool(), Found: findDeviceInfoTool() != "", Kind: "info"},
			{Name: "ideviceinstaller", Path: findInstallTool(), Found: findInstallTool() != "", Kind: "install"},
		},
		"listError":      listErr,
		"hostOS":         runtime.GOOS,
		"infoTool":       infoTool,
		"installTool":    installTool,
		"toolsAvailable": installTool != "",
	})
}

// installUploadMemory limits the amount of an IPA kept in RAM before it is
// streamed to a temporary file on disk.
const installUploadMemory = 64 << 20

func handleAPIInstallUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(installUploadMemory); err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse upload: %v", err))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	udid := strings.TrimSpace(r.FormValue("udid"))
	deviceName := strings.TrimSpace(r.FormValue("deviceName"))
	if udid == "" {
		jsonError(w, http.StatusBadRequest, "device UDID is required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "an .ipa file is required")
		return
	}
	defer file.Close()

	if !strings.EqualFold(filepath.Ext(header.Filename), ".ipa") {
		jsonError(w, http.StatusBadRequest, "only .ipa files can be installed")
		return
	}

	installer := findInstallTool()
	if installer == "" {
		jsonError(w, http.StatusServiceUnavailable,
			"ideviceinstaller is not installed. Install libimobiledevice (e.g. on macOS: brew install libimobiledevice, on Windows: iDevice Suite / libimobiledevice Windows build) and reconnect the device.")
		return
	}

	dir := filepath.Join(os.TempDir(), "ipatool-gui-installs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create upload directory: %v", err))
		return
	}

	tmp, err := os.CreateTemp(dir, "install-*.ipa")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create temporary file: %v", err))
		return
	}
	finalPath := tmp.Name()

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		_ = os.Remove(finalPath)
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save uploaded file: %v", err))
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(finalPath)
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to close uploaded file: %v", err))
		return
	}

	job := &installJob{
		ID:         fmt.Sprintf("install_%d", time.Now().UnixNano()),
		UDID:       udid,
		DeviceName: deviceName,
		FileName:   filepath.Base(header.Filename),
		FilePath:   finalPath,
		Status:     "queued",
		CreatedAt:  time.Now().Unix(),
	}
	installJobs.add(job)

	go executeInstallJob(job, installer, udid)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"jobId":   job.ID,
	})
}

func executeInstallJob(job *installJob, installer, udid string) {
	installJobs.update(job.ID, func(j *installJob) {
		j.Status = "installing"
		j.Message = "Подключение и установка .IPA на устройство..."
	})

	cmd := exec.Command(installer, "-u", udid, "install", job.FilePath)

	writer := &installLogWriter{
		jobID: job.ID,
	}
	cmd.Stdout = writer
	cmd.Stderr = writer

	err := cmd.Run()

	_ = os.Remove(job.FilePath)

	// Read the full log before acquiring the tracker lock to avoid holding two
	// locks at once while the writer may still be emitting data.
	log := writer.String()

	installJobs.update(job.ID, func(j *installJob) {
		j.Log = log
		if err != nil {
			j.Status = "error"
			j.Error = err.Error()
			j.Message = "Ошибка установки"
			return
		}
		j.Status = "completed"
		j.Progress = 100.0
		j.Message = "Приложение успешно установлено"
	})
}

// installLogWriter accumulates command output and copies the latest line to the
// job message so the GUI can show simple, non-blocking install progress.
type installLogWriter struct {
	jobID string
	mu    sync.Mutex
	buf   strings.Builder
}

func (w *installLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	w.buf.Write(p)
	lines := strings.Split(w.buf.String(), "\n")
	line := ""
	if len(lines) > 0 {
		line = strings.TrimSpace(lines[len(lines)-1])
		// ideviceinstaller uses carriage returns for in-place progress updates.
		if parts := strings.Split(line, "\r"); len(parts) > 1 {
			line = strings.TrimSpace(parts[len(parts)-1])
		}
	}
	log := w.buf.String()
	w.mu.Unlock()

	installJobs.update(w.jobID, func(j *installJob) {
		j.Log = log
		if line != "" && j.Status != "completed" && j.Status != "error" {
			j.Message = line
		}
	})
	return len(p), nil
}

func (w *installLogWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func handleAPIInstallStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	jobID := r.URL.Query().Get("jobId")
	if jobID == "" {
		jsonError(w, http.StatusBadRequest, "jobId is required")
		return
	}

	job, found := installJobs.get(jobID)
	if !found {
		jsonError(w, http.StatusNotFound, "install job not found")
		return
	}

	jsonResponse(w, http.StatusOK, job)
}


