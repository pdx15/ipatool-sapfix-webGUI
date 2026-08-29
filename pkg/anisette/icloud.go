package anisette

// ICloudStatus describes whether a compatible iCloud for Windows installation
// was found on the machine, which variant it is, and where to download it.
type ICloudStatus struct {
	Installed   bool   `json:"installed"`
	Variant     string `json:"variant"` // "classic", "microsoft-store", ""
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
}

// CheckICloud detects a locally installed iCloud for Windows.
func CheckICloud() ICloudStatus {
	return checkICloud()
}
