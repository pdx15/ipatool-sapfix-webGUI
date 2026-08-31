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
	ProductName    string `json:"productName,omitempty"`
	ModelName      string `json:"modelName,omitempty"`
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

// findToolInAppBundle checks the tools folder shipped next to the executable and
// the current working directory. It is used so Windows users can simply drop
// ideviceinstaller.exe, idevice_id.exe and idevicedeviceinfo.exe into the tools
// directory without adding anything to PATH.
func findToolInAppBundle(name string) string {
	var candidates []string
	candidates = append(candidates, filepath.Join("tools", name), filepath.Join("tools", name+".exe"))

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "tools", name),
			filepath.Join(exeDir, "tools", name+".exe"),
			filepath.Join(exeDir, name),
			filepath.Join(exeDir, name+".exe"),
		)
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "tools", name),
			filepath.Join(wd, "tools", name+".exe"),
		)
	}

	seen := make(map[string]bool)
	for _, path := range candidates {
		if seen[path] {
			continue
		}
		seen[path] = true
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}

	return ""
}

func findInstallTool() string {
	if path := os.Getenv("IPATOOL_IDEVICEINSTALLER"); path != "" {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path
		}
	}

	if path := findToolInAppBundle("ideviceinstaller"); path != "" {
		return path
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

	if path := findToolInAppBundle("idevice_id"); path != "" {
		return path
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

	if path := findToolInAppBundle("idevicedeviceinfo"); path != "" {
		return path
	}
	// Older/alternative builds ship ideviceinfo instead of idevicedeviceinfo.
	if path := findToolInAppBundle("ideviceinfo"); path != "" {
		return path
	}

	for _, name := range []string{"idevicedeviceinfo", "idevicedeviceinfo.exe", "ideviceinfo", "ideviceinfo.exe"} {
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
			`C:\Program Files\idevice\ideviceinfo.exe`,
			`C:\Program Files (x86)\idevice\ideviceinfo.exe`,
			`C:\Program Files\libimobiledevice\ideviceinfo.exe`,
			`C:\Program Files (x86)\libimobiledevice\ideviceinfo.exe`,
		}
	} else {
		candidates = []string{
			"/opt/homebrew/bin/idevicedeviceinfo",
			"/usr/local/bin/idevicedeviceinfo",
			"/opt/local/bin/idevicedeviceinfo",
			"/usr/bin/idevicedeviceinfo",
			"/opt/homebrew/bin/ideviceinfo",
			"/usr/local/bin/ideviceinfo",
			"/opt/local/bin/ideviceinfo",
			"/usr/bin/ideviceinfo",
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
			info := readInstallDeviceInfo(infoTool, udid)
			device.Name = firstNonEmpty(
				info["DeviceName"],
				info["ProductName"],
				info["MarketingName"],
			)
			device.ProductType = info["ProductType"]
			device.ProductName = info["ProductName"]
			device.ProductVersion = info["ProductVersion"]
			device.SerialNumber = info["SerialNumber"]
			device.ModelName = deviceModelName(info["ProductType"], info["ProductName"], device.Name)
		}
		devices = append(devices, device)
	}

	return devices, "", infoTool, installTool
}

func readInstallDeviceInfo(tool, udid string) map[string]string {
	info := make(map[string]string)

	// Most builds use "-u UDID"; some only accept "-u UDID" as the only
	// argument or "-o" for pairing. Try the full output first.
	raw, _ := runInstallCommand(5*time.Second, tool, "-u", udid)
	for _, line := range strings.Split(raw, "\n") {
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if key != "" && value != "" {
				info[key] = value
			}
		}
	}

	// Some builds return empty unless "-k" is supplied per key.
	for _, key := range []string{"DeviceName", "ProductName", "ProductType", "ProductVersion", "SerialNumber"} {
		if info[key] == "" {
			value, _ := runInstallCommand(3*time.Second, tool, "-u", udid, "-k", key)
			if strings.TrimSpace(value) != "" {
				info[key] = strings.TrimSpace(value)
			}
		}
	}

	return info
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// deviceModelName maps Apple product type / product name strings to a friendly
// user-facing model label. Unknown identifiers fall back to the raw values.
func deviceModelName(productType, productName, name string) string {
	if name != "" {
		return name
	}

	product := strings.ToLower(strings.TrimSpace(productType))
	productName = strings.TrimSpace(productName)

	byProductType := map[string]string{
		"iphone9,1": "iPhone 7",
		"iphone9,2": "iPhone 7 Plus",
		"iphone9,3": "iPhone 7",
		"iphone9,4": "iPhone 7 Plus",
		"iphone10,1": "iPhone 8",
		"iphone10,2": "iPhone 8 Plus",
		"iphone10,3": "iPhone X",
		"iphone10,4": "iPhone 8",
		"iphone10,5": "iPhone 8 Plus",
		"iphone10,6": "iPhone X",
		"iphone11,2": "iPhone XS",
		"iphone11,4": "iPhone XS Max",
		"iphone11,6": "iPhone XS Max",
		"iphone11,8": "iPhone XR",
		"iphone12,1": "iPhone 11",
		"iphone12,3": "iPhone 11 Pro",
		"iphone12,5": "iPhone 11 Pro Max",
		"iphone12,8": "iPhone SE (2nd generation)",
		"iphone13,1": "iPhone 12 mini",
		"iphone13,2": "iPhone 12",
		"iphone13,3": "iPhone 12 Pro",
		"iphone13,4": "iPhone 12 Pro Max",
		"iphone14,2": "iPhone 13 Pro",
		"iphone14,3": "iPhone 13 Pro Max",
		"iphone14,4": "iPhone 13 mini",
		"iphone14,5": "iPhone 13",
		"iphone14,7": "iPhone SE (3rd generation)",
		"iphone15,2": "iPhone 14 Pro",
		"iphone15,3": "iPhone 14 Pro Max",
		"iphone15,4": "iPhone 14",
		"iphone15,5": "iPhone 14 Plus",
		"iphone16,1": "iPhone 15 Pro",
		"iphone16,2": "iPhone 15 Pro Max",
		"iphone16,3": "iPhone 15",
		"iphone16,4": "iPhone 15 Plus",
		"iphone17,1": "iPhone 16 Pro",
		"iphone17,2": "iPhone 16 Pro Max",
		"iphone17,3": "iPhone 16",
		"iphone17,4": "iPhone 16 Plus",
		"iphone17,5": "iPhone 16e",
		"iphone1,1": "iPhone",
		"iphone1,2": "iPhone 3G",
		"iphone2,1": "iPhone 3GS",
		"iphone3,1": "iPhone 4",
		"iphone3,2": "iPhone 4",
		"iphone3,3": "iPhone 4 CDMA",
		"iphone4,1": "iPhone 4S",
		"iphone5,1": "iPhone 5",
		"iphone5,2": "iPhone 5",
		"iphone5,3": "iPhone 5c",
		"iphone5,4": "iPhone 5c",
		"iphone6,1": "iPhone 5s",
		"iphone6,2": "iPhone 5s",
		"iphone7,1": "iPhone 6 Plus",
		"iphone7,2": "iPhone 6",
		"iphone8,1": "iPhone 6s",
		"iphone8,2": "iPhone 6s Plus",
		"iphone8,4": "iPhone SE (1st generation)",
	}

	if productType == "" {
		if productName != "" {
			if strings.Contains(strings.ToLower(productName), "iphone") {
				return productName
			}
			if strings.Contains(strings.ToLower(productName), "ipad") {
				return productName
			}
			if strings.Contains(strings.ToLower(productName), "tv") {
				return productName
			}
		}
		return ""
	}

	if model, ok := byProductType[product]; ok {
		return model
	}

	if strings.HasPrefix(product, "ipad") || strings.HasPrefix(product, "iphone") || strings.HasPrefix(product, "watch") || strings.HasPrefix(product, "appletv") {
		if productName != "" {
			return productName
		}
		return strings.ToUpper(productType)
	}

	return productName
}

// checkAppleMobileDeviceSupport reports whether the USB driver/service required
// to talk to iOS devices over Windows is installed. On macOS/Linux it is
// considered already available because libimobiledevice does not need it.
func checkAppleMobileDeviceSupport() map[string]interface{} {
	if runtime.GOOS != "windows" {
		return map[string]interface{}{
			"installed":  true,
			"required":   false,
			"message":    "Проверка драйвера Apple Mobile Device Support требуется только на Windows.",
			"downloadUrl": "",
		}
	}

	var paths []string
	programFiles := os.Getenv("ProgramFiles")
	if programFiles == "" {
		programFiles = `C:\Program Files`
	}
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	if programFilesX86 == "" {
		programFilesX86 = `C:\Program Files (x86)`
	}

	common64 := filepath.Join(programFiles, "Common Files", "Apple", "Mobile Device Support")
	common86 := filepath.Join(programFilesX86, "Common Files", "Apple", "Mobile Device Support")
	itunes64 := filepath.Join(programFiles, "iTunes", "iTunes.exe")
	itunes86 := filepath.Join(programFilesX86, "iTunes", "iTunes.exe")

	paths = append(paths,
		filepath.Join(common64, "MobileDevice.dll"),
		filepath.Join(common64, "drivers", "usbaapl64.sys"),
		filepath.Join(common64, "usbaapl64.sys"),
		filepath.Join(common86, "MobileDevice.dll"),
		filepath.Join(common86, "drivers", "usbaapl64.sys"),
		filepath.Join(common86, "usbaapl64.sys"),
		itunes64,
		itunes86,
	)

	foundPath := ""
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			foundPath = p
			break
		}
	}

	// Also check the Apple Mobile Device service (newer iTunes still installs
	// "Apple Mobile Device Service").
	serviceInstalled := false
	for _, service := range []string{"Apple Mobile Device Service", "Apple Mobile Device"} {
		_ = runInstallCommand(2*time.Second, "sc", "query", service)
		// runInstallCommand does not return status, so check via exec directly.
		if _, ok := queryWindowsService(service); ok {
			serviceInstalled = true
			break
		}
	}

	installed := foundPath != "" || serviceInstalled

	return map[string]interface{}{
		"installed": installed,
		"required":  true,
		"path":      foundPath,
		"service":   serviceInstalled,
		"message":   "Драйвер и служба Apple Mobile Device Support не найдены. Установите iTunes (встроенный Apple Mobile Device Support) или обновите драйвер USB.",
		"downloadUrl": "https://support.apple.com/en-us/HT210384",
		"itunesUrl":   "https://www.apple.com/itunes/",
	}
}

func queryWindowsService(service string) (string, bool) {
	out, err := runInstallCommand(2*time.Second, "sc", "query", service)
	if err != nil {
		// sc may exit non-zero for "not found"; that is fine.
		return "", false
	}
	return out, strings.Contains(strings.ToLower(out), "running") || strings.Contains(strings.ToLower(out), "stopped")
}

func handleAPIInstallDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	devices, listErr, infoTool, installerTool := listInstallDevices()

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"devices": devices,
		"tools": []installTool{
			{Name: "idevice_id", Path: findDeviceListTool(), Found: findDeviceListTool() != "", Kind: "list"},
			{Name: "idevicedeviceinfo", Path: findDeviceInfoTool(), Found: findDeviceInfoTool() != "", Kind: "info"},
			{Name: "ideviceinstaller", Path: findInstallTool(), Found: findInstallTool() != "", Kind: "install"},
		},
		"driver":         checkAppleMobileDeviceSupport(),
		"listError":      listErr,
		"hostOS":         runtime.GOOS,
		"infoTool":       infoTool,
		"installTool":    installerTool,
		"toolsAvailable": installerTool != "",
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


