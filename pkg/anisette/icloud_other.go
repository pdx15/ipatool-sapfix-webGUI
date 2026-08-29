//go:build !windows

package anisette

func checkICloud() ICloudStatus {
	return ICloudStatus{
		DownloadURL: "https://support.apple.com/en-us/103121",
	}
}
