//go:build !windows

package anisette

const icloudClassicDownloadURL = "https://updates.cdn-apple.com/2020/windows/001-39935-20200911-1A70AA56-F448-11EA-8CC0-99D41950005E/iCloudSetup.exe"

func checkICloud() ICloudStatus {
	return ICloudStatus{
		DownloadURL: icloudClassicDownloadURL,
	}
}
