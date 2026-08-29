//go:build windows && amd64

package anisette

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Microsoft Store iCloud anisette extraction, ported from the reference
// implementation in the native ipatool engine (anisette.cpp). It loads
// AOSKit.dll from a cached copy of the WindowsApps package and calls
// copyOTPHeadersForDSID via offsets pinned to AOSKit 15.9.60.0 (133.3).
//
// On amd64 the Microsoft x64 calling convention is uniform, so raw function
// pointers (including __fastcall) can be invoked through syscall.SyscallN.

const (
	// Offsets relative to AOSRegisterClientInfo (AOSKit 15.9.60.0 = 133.3).
	ofsCopyOTP       = 0x115E0
	ofsGetDeviceID   = 0x134E0
	ofsGetLocalUUID  = 0x12EC0
	ofsClientInfoOS  = 0x161B0
	ofsClientInfoMdl = 0x163C0

	kCFStringEncodingUTF8 = 0x08000100

	msstoreClientInfo = "<PC> <Windows;6.2(0,0);9200> <com.apple.AuthKitWin/1 (com.apple.iCloud/1)>"
	msstoreSerial     = "C02LKHBBFD57"
	msstoreRouting    = "67437824"
	msstoreUserAgent  = "iTunes/12.13.10 (Windows; Microsoft Windows 10 x64 Professional Edition (Build 19045); x64) AppleWebKit/7613.3.9.0.2"
)

// fetchMSStore extracts anisette data from a Microsoft Store iCloud install.
func fetchMSStore() (Data, error) {
	icloudDir, err := findICloudDir()
	if err != nil {
		return Data{}, err
	}

	cacheDir, err := prepareDLLCache(icloudDir)
	if err != nil {
		return Data{}, err
	}

	adiDir, err := findADIDir()
	if err != nil {
		return Data{}, err
	}

	setupRegistry(cacheDir, adiDir)

	if err := windows.SetDllDirectory(cacheDir); err != nil {
		return Data{}, fmt.Errorf("anisette: SetDllDirectory failed: %w", err)
	}

	if err := windows.SetCurrentDirectory(windows.StringToUTF16Ptr(cacheDir)); err != nil {
		return Data{}, fmt.Errorf("anisette: SetCurrentDirectory failed: %w", err)
	}

	// Load the Apple CoreFoundation/objc runtime DLLs from the cache.
	for _, name := range []string{
		"CoreFoundation.dll",
		"CoreADI64.dll",
		"objc.dll",
		"Foundation.dll",
		"libdispatch.dll",
		"CFNetwork.dll",
	} {
		if _, err := windows.LoadLibrary(filepath.Join(cacheDir, name)); err != nil {
			// Non-fatal: not every package ships every DLL.
			continue
		}
	}

	if aplzod, err := windows.LoadLibrary(filepath.Join(cacheDir, "APLZOD6432.dll")); err == nil {
		if msInit, err := windows.GetProcAddress(aplzod, "MSProviderInit"); err == nil {
			_, _, _ = syscall.SyscallN(msInit)
		}
	}

	aosKit, err := windows.LoadLibrary(filepath.Join(cacheDir, "AOSKit.dll"))
	if err != nil {
		return Data{}, errors.New("anisette: failed to load AOSKit.dll from cache")
	}

	// Register client info.
	if aosReg, err := windows.GetProcAddress(aosKit, "AOSRegisterClientInfo"); err == nil {
		args := []uintptr{
			strPtr("com.apple.iCloud"),
			strPtr("15.8"),
			strPtr("com.apple.AuthKitWin"),
			strPtr("1"),
		}
		_, _, _ = syscall.SyscallN(aosReg, args...)
	}

	anchor, err := windows.GetProcAddress(aosKit, "AOSRegisterClientInfo")
	if err != nil {
		return Data{}, errors.New("anisette: AOSRegisterClientInfo not found in AOSKit.dll")
	}

	copyOTP := anchor + ofsCopyOTP
	getDeviceID := anchor + ofsGetDeviceID
	getLocalUUID := anchor + ofsGetLocalUUID

	// Exported dictionary keys (data symbols holding CFStringRef pointers).
	keyMDM, err := exportedDataPointer(aosKit, "kAOSMDMachineIdHeaderName")
	if err != nil {
		return Data{}, errors.New("anisette: AOSKit MD keys not found")
	}

	keyMD, err := exportedDataPointer(aosKit, "kAOSMDOneTimePasswordHeaderName")
	if err != nil {
		return Data{}, errors.New("anisette: AOSKit MD keys not found")
	}

	// CoreFoundation functions.
	cf, err := windows.LoadLibrary(filepath.Join(cacheDir, "CoreFoundation.dll"))
	if err != nil {
		return Data{}, errors.New("anisette: CoreFoundation.dll not loaded")
	}

	cfCreate, err := windows.GetProcAddress(cf, "CFStringCreateWithCString")
	if err != nil {
		return Data{}, errors.New("anisette: CoreFoundation exports missing")
	}

	cfGetLen, err := windows.GetProcAddress(cf, "CFStringGetLength")
	if err != nil {
		return Data{}, errors.New("anisette: CoreFoundation exports missing")
	}

	cfGetStr, err := windows.GetProcAddress(cf, "CFStringGetCString")
	if err != nil {
		return Data{}, errors.New("anisette: CoreFoundation exports missing")
	}

	cfDictGet, err := windows.GetProcAddress(cf, "CFDictionaryGetValue")
	if err != nil {
		return Data{}, errors.New("anisette: CoreFoundation exports missing")
	}

	cfRelease, err := windows.GetProcAddress(cf, "CFRelease")
	if err != nil {
		return Data{}, errors.New("anisette: CoreFoundation exports missing")
	}

	// call copyOTPHeadersForDSID("-2"/"-1") to obtain the OTP headers dict.
	var dict uintptr
	for _, dsid := range []string{"-2", "-1"} {
		dsidStr, _, _ := syscall.SyscallN(cfCreate, 0, strPtr(dsid), kCFStringEncodingUTF8)
		if dsidStr == 0 {
			continue
		}

		dict, _, _ = syscall.SyscallN(copyOTP, dsidStr)
		_, _, _ = syscall.SyscallN(cfRelease, dsidStr)
		if dict != 0 {
			break
		}
	}

	if dict == 0 {
		return Data{}, errors.New("anisette: copyOTPHeadersForDSID returned null — is iCloud signed in?")
	}

	otp := cfDictString(cfDictGet, cfGetLen, cfGetStr, cfRelease, dict, keyMD)
	machineID := cfDictString(cfDictGet, cfGetLen, cfGetStr, cfRelease, dict, keyMDM)

	_, _, _ = syscall.SyscallN(cfRelease, dict)

	if otp == "" || machineID == "" {
		return Data{}, errors.New("anisette: empty OTP/MachineID from AOSKit")
	}

	deviceID, _, _ := syscall.SyscallN(getDeviceID)
	localUUID, _, _ := syscall.SyscallN(getLocalUUID)

	return Data{
		OTP:           otp,
		MachineID:     machineID,
		LocalUserUUID: charPtrToString(localUUID),
		DeviceID:      charPtrToString(deviceID),
		ClientInfo:    msstoreClientInfo,
		SerialNo:      msstoreSerial,
		RoutingInfo:   msstoreRouting,
		Locale:        "en_US",
		Timezone:      "PST",
		UserAgent:     msstoreUserAgent,
	}, nil
}

// exportedDataPointer returns the CFStringRef stored in an exported data
// symbol (kAOSMDMachineIdHeaderName etc.).
func exportedDataPointer(module windows.Handle, name string) (uintptr, error) {
	addr, err := windows.GetProcAddress(module, name)
	if err != nil {
		return 0, err
	}

	return *(*uintptr)(unsafe.Pointer(addr)), nil
}

func cfDictString(cfDictGet, cfGetLen, cfGetStr, cfRelease, dict, key uintptr) string {
	val, _, _ := syscall.SyscallN(cfDictGet, dict, key)
	if val == 0 {
		return ""
	}

	defer func() { _, _, _ = syscall.SyscallN(cfRelease, val) }()

	// Skip empty strings cheaply.
	if n, _, _ := syscall.SyscallN(cfGetLen, val); n == 0 {
		return ""
	}

	buf := make([]byte, 2048)
	ok, _, _ := syscall.SyscallN(cfGetStr, val, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), kCFStringEncodingUTF8)
	if ok == 0 {
		return ""
	}

	return cString(buf)
}

func strPtr(s string) uintptr {
	p, err := windows.BytePtrFromString(s)
	if err != nil {
		panic(err)
	}

	return uintptr(unsafe.Pointer(p))
}

func charPtrToString(p uintptr) string {
	if p == 0 {
		return ""
	}

	return windows.BytePtrToString((*byte)(unsafe.Pointer(p)))
}

// findICloudDir locates the newest Microsoft Store iCloud package that ships
// AOSKit.dll under WindowsApps.
func findICloudDir() (string, error) {
	root := `C:\Program Files\WindowsApps`
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("anisette: WindowsApps not found: %w", err)
	}

	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "AppleInc.iCloud_") {
			continue
		}

		icloud := filepath.Join(root, entry.Name(), "iCloud")
		if _, err := os.Stat(filepath.Join(icloud, "AOSKit.dll")); err == nil {
			candidates = append(candidates, icloud)
		}
	}

	if len(candidates) == 0 {
		return "", errors.New("anisette: MS Store iCloud not found — install iCloud from the Microsoft Store")
	}

	sort.Strings(candidates)
	return candidates[len(candidates)-1], nil
}

// prepareDLLCache copies the package DLLs into a version-pinned cache under
// LocalAppData. The cache is intentionally not invalidated on iCloud updates,
// because the offsets are pinned to a specific AOSKit version.
func prepareDLLCache(icloudDir string) (string, error) {
	local, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return "", err
	}

	cache := filepath.Join(local, "IPA_Downloader", "anisette-cache", filepath.Base(filepath.Dir(icloudDir)))
	if _, err := os.Stat(filepath.Join(cache, "AOSKit.dll")); err == nil {
		return cache, nil
	}

	if err := os.MkdirAll(cache, 0755); err != nil {
		return "", err
	}

	entries, err := os.ReadDir(icloudDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".dll" {
			continue
		}

		src := filepath.Join(icloudDir, entry.Name())
		dst := filepath.Join(cache, entry.Name())
		if err := copyFile(src, dst); err != nil {
			return "", err
		}
	}

	if _, err := os.Stat(filepath.Join(cache, "AOSKit.dll")); err != nil {
		return "", errors.New("anisette: failed to prepare DLL cache")
	}

	return cache, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// findADIDir locates the adi.pb provisioning file.
func findADIDir() (string, error) {
	classic := `C:\ProgramData\Apple Computer\iTunes\adi`
	if _, err := os.Stat(filepath.Join(classic, "adi.pb")); err == nil {
		return classic, nil
	}

	if local, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0); err == nil {
		p := filepath.Join(local, "Apple", "Internet Services", "adi")
		if _, err := os.Stat(filepath.Join(p, "adi.pb")); err == nil {
			return p, nil
		}
	}

	if roaming, err := windows.KnownFolderPath(windows.FOLDERID_RoamingAppData, 0); err == nil {
		p := filepath.Join(roaming, "Apple Computer", "iTunes", "adi")
		if _, err := os.Stat(filepath.Join(p, "adi.pb")); err == nil {
			return p, nil
		}
	}

	return "", errors.New("anisette: adi.pb not found — sign in to iCloud with your Apple ID first")
}

// setupRegistry points CoreADI at the cached DLLs and the adi directory.
func setupRegistry(dllDir, adiDir string) {
	installDir := strings.TrimRight(dllDir, `\`) + `\`

	entries := []struct {
		root   registry.Key
		subkey string
		name   string
		value  string
	}{
		{registry.CURRENT_USER, `SOFTWARE\Apple Inc.\Apple Application Support`, "InstallDir", installDir},
		{registry.CURRENT_USER, `SOFTWARE\Apple Inc.\Internet Services`, "InstallDir", installDir},
		{registry.CURRENT_USER, `SOFTWARE\Apple Inc.\CoreADI`, "ADIPath", adiDir},
		{registry.LOCAL_MACHINE, `SOFTWARE\Apple Inc.\Apple Application Support`, "InstallDir", installDir},
		{registry.LOCAL_MACHINE, `SOFTWARE\Apple Inc.\Internet Services`, "InstallDir", installDir},
		{registry.LOCAL_MACHINE, `SOFTWARE\Apple Inc.\CoreADI`, "ADIPath", adiDir},
	}

	for _, e := range entries {
		k, _, err := registry.CreateKey(e.root, e.subkey, registry.SET_VALUE)
		if err != nil {
			continue
		}

		_ = k.SetStringValue(e.name, e.value)
		_ = k.Close()
	}
}
