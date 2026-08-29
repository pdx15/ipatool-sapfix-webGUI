//go:build windows && cgo

package anisette

/*
#cgo windows LDFLAGS: -lole32 -lshell32

#include <windows.h>
#include <shlobj.h>
#include <objbase.h>
#include <stdio.h>
#include <wchar.h>
#include <string.h>

// ── Objective-C runtime (classic iCloud) ─────────────────────────────────
// Classic "2020" iCloud for Windows ships the GNUstep Objective-C runtime
// (objc.dll) plus Foundation.dll and AOSKit.dll under Common Files\Apple.
// Anisette is obtained by calling +[AOSUtilities retrieveOTPHeadersForDSID:],
// which returns an NSDictionary keyed by "X-Apple-MD" (OTP) and
// "X-Apple-MD-M" (machine ID). This path uses no raw offsets, so it is stable
// across classic iCloud builds — that is why it is preferred (P1).

typedef void* objc_id;
typedef void* objc_sel;

typedef objc_id  (__cdecl *objc_getClass_fn)(const char* name);
typedef objc_sel (__cdecl *sel_registerName_fn)(const char* name);
typedef objc_id  (__cdecl *objc_msgSend_fn)(objc_id self, objc_sel sel, ...);

static objc_id ns_string(objc_msgSend_fn msgsend, objc_getClass_fn getclass,
                         sel_registerName_fn selreg, const char* s) {
    objc_id cls = getclass("NSString");
    objc_sel sel = selreg("stringWithUTF8String:");
    if (!cls || !sel) return NULL;
    objc_id (*fn)(objc_id, objc_sel, const char*) =
        (objc_id (*)(objc_id, objc_sel, const char*))msgsend;
    return fn(cls, sel, s);
}

static objc_id dict_get(objc_msgSend_fn msgsend, sel_registerName_fn selreg,
                        objc_id dict, objc_id key) {
    objc_sel sel = selreg("objectForKey:");
    if (!dict || !key || !sel) return NULL;
    objc_id (*fn)(objc_id, objc_sel, objc_id) =
        (objc_id (*)(objc_id, objc_sel, objc_id))msgsend;
    return fn(dict, sel, key);
}

static const char* objc_utf8(objc_msgSend_fn msgsend, sel_registerName_fn selreg,
                             objc_id str) {
    objc_sel sel = selreg("UTF8String");
    if (!str || !sel) return NULL;
    const char* (*fn)(objc_id, objc_sel) =
        (const char* (*)(objc_id, objc_sel))msgsend;
    return fn(str, sel);
}

// copy_nsstring returns the UTF-8 contents of an NSString (or objc object
// responding to UTF8String) into buf, or 0 when it is missing/empty.
static int copy_nsstring(objc_msgSend_fn msgsend, sel_registerName_fn selreg,
                         objc_id str, char* buf, int cap) {
    const char* s = objc_utf8(msgsend, selreg, str);
    if (!s || !*s) return 0;
    snprintf(buf, cap, "%s", s);
    return 1;
}

// classic_fetch extracts OTP + machine ID from classic iCloud installed under
// Common Files\Apple. supportDir holds objc.dll/Foundation.dll; servicesDir
// holds AOSKit.dll. Returns 0 on success.
static int classic_fetch(const wchar_t* support_dir, const wchar_t* services_dir,
                         char* otp, int otp_cap, char* mid, int mid_cap,
                         char* serial, int serial_cap,
                         char* udid, int udid_cap,
                         int* win_err) {
    wchar_t objc_path[MAX_PATH];
    wchar_t foundation_path[MAX_PATH];
    wchar_t aoskit_path[MAX_PATH];

    swprintf(objc_path, MAX_PATH, L"%ls\\objc.dll", support_dir);
    swprintf(foundation_path, MAX_PATH, L"%ls\\Foundation.dll", support_dir);
    swprintf(aoskit_path, MAX_PATH, L"%ls\\AOSKit.dll", services_dir);

    // objc.dll is a GNUstep Objective-C runtime whose import dependencies
    // (Foundation.dll, CoreFoundation.dll, libdispatch.dll, ...) live in the
    // same Apple Application Support directory. Without setting the DLL search
    // directory the loader cannot resolve those dependencies, so
    // LoadLibraryW(objc.dll) fails. The AltServer reference implementation
    // changes the working directory to support_dir before loading; do the same
    // via SetDllDirectoryW, which also covers AOSKit.dll's own dependencies.
    SetDllDirectoryW(support_dir);

    HMODULE objc_lib = LoadLibraryW(objc_path);
    if (!objc_lib) { if (win_err) *win_err = (int)GetLastError(); return -1; }
    HMODULE foundation_lib = LoadLibraryW(foundation_path);
    if (!foundation_lib) { if (win_err) *win_err = (int)GetLastError(); return -2; }
    HMODULE aoskit_lib = LoadLibraryW(aoskit_path);
    if (!aoskit_lib) { if (win_err) *win_err = (int)GetLastError(); return -3; }

    objc_getClass_fn getclass = (objc_getClass_fn)GetProcAddress(objc_lib, "objc_getClass");
    sel_registerName_fn selreg = (sel_registerName_fn)GetProcAddress(objc_lib, "sel_registerName");
    objc_msgSend_fn msgsend = (objc_msgSend_fn)GetProcAddress(objc_lib, "objc_msgSend");
    if (!getclass || !selreg || !msgsend) return -3;

    objc_id dsid = ns_string(msgsend, getclass, selreg, "-2");
    objc_id key_md = ns_string(msgsend, getclass, selreg, "X-Apple-MD");
    objc_id key_mdm = ns_string(msgsend, getclass, selreg, "X-Apple-MD-M");
    if (!dsid || !key_md || !key_mdm) return -4;

    objc_id aos_utilities = getclass("AOSUtilities");
    if (!aos_utilities) return -5;

    // Mirror AltServer: AOSUtilities exposes +machineSerialNumber and
    // +machineUDID on macOS, but the Windows classic AOSKit does NOT ship them
    // (calling them raises "unrecognized selector"). sel_registerName always
    // returns a non-NULL selector, so guard every call with respondsToSelector:
    // and fall back to the constant values when the methods are absent.
    objc_sel responds_sel = selreg("respondsToSelector:");
    objc_sel serial_sel = selreg("machineSerialNumber");
    objc_sel udid_sel = selreg("machineUDID");
    if (responds_sel) {
        int (*responds_fn)(objc_id, objc_sel, objc_sel) =
            (int (*)(objc_id, objc_sel, objc_sel))msgsend;
        if (serial_sel && responds_fn(aos_utilities, responds_sel, serial_sel)) {
            objc_id (*serial_fn)(objc_id, objc_sel) =
                (objc_id (*)(objc_id, objc_sel))msgsend;
            copy_nsstring(msgsend, selreg, serial_fn(aos_utilities, serial_sel),
                          serial, serial_cap);
        }
        if (udid_sel && responds_fn(aos_utilities, responds_sel, udid_sel)) {
            objc_id (*udid_fn)(objc_id, objc_sel) =
                (objc_id (*)(objc_id, objc_sel))msgsend;
            copy_nsstring(msgsend, selreg, udid_fn(aos_utilities, udid_sel),
                          udid, udid_cap);
        }
    }

    objc_sel retrieve_sel = selreg("retrieveOTPHeadersForDSID:");
    if (!retrieve_sel) return -6;
    objc_id (*retrieve)(objc_id, objc_sel, objc_id) =
        (objc_id (*)(objc_id, objc_sel, objc_id))msgsend;
    objc_id headers = retrieve(aos_utilities, retrieve_sel, dsid);
    if (!headers) return -7;

    objc_id otp_obj = dict_get(msgsend, selreg, headers, key_md);
    objc_id mid_obj = dict_get(msgsend, selreg, headers, key_mdm);
    if (!otp_obj || !mid_obj) return -8;

    const char* otp_str = objc_utf8(msgsend, selreg, otp_obj);
    const char* mid_str = objc_utf8(msgsend, selreg, mid_obj);
    if (!otp_str || !mid_str) return -9;

    snprintf(otp, otp_cap, "%s", otp_str);
    snprintf(mid, mid_cap, "%s", mid_str);
    return 0;
}

// FOLDERID_ProgramFilesCommon and FOLDERID_ProgramFilesCommonX86 are declared
// extern in mingw's shlobj.h but their definitions are not emitted unless
// INITGUID is defined before including the headers, which causes an
// "undefined reference" at link time. Define them here as plain GUID constants
// with the standard well-known values so the build works with any mingw.
static const GUID kFolderIdProgramFilesCommon = {
    0xF7F1ED05, 0x9F6D, 0x47A2, {0xAA, 0xAE, 0x29, 0xD3, 0x17, 0xC6, 0xF0, 0x66}};
static const GUID kFolderIdProgramFilesCommonX86 = {
    0xDE974D24, 0xD9C6, 0x4D3E, {0xBF, 0x91, 0xF4, 0x45, 0x51, 0x20, 0xB9, 0x17}};

// ipatool_anisette_classic tries the 64-bit and 32-bit Common Files locations
// in turn, whichever LoadLibrary accepts for this process's architecture.
// Returns 0 on success.
int ipatool_anisette_classic(char* otp, int otp_cap, char* mid, int mid_cap,
                             char* serial, int serial_cap,
                             char* udid, int udid_cap,
                             int* win_err) {
    wchar_t support[MAX_PATH];
    wchar_t services[MAX_PATH];

    if (win_err) *win_err = 0;

    wchar_t* pf = NULL;
    wchar_t* pfx86 = NULL;
    SHGetKnownFolderPath(&kFolderIdProgramFilesCommon, 0, NULL, &pf);
    SHGetKnownFolderPath(&kFolderIdProgramFilesCommonX86, 0, NULL, &pfx86);

    if (pf) {
        swprintf(support, MAX_PATH, L"%ls\\Apple\\Apple Application Support", pf);
        swprintf(services, MAX_PATH, L"%ls\\Apple\\Internet Services", pf);
        CoTaskMemFree(pf);
        if (classic_fetch(support, services, otp, otp_cap, mid, mid_cap,
                          serial, serial_cap, udid, udid_cap, win_err) == 0) {
            if (pfx86) CoTaskMemFree(pfx86);
            return 0;
        }
    }

    if (pfx86) {
        swprintf(support, MAX_PATH, L"%ls\\Apple\\Apple Application Support", pfx86);
        swprintf(services, MAX_PATH, L"%ls\\Apple\\Internet Services", pfx86);
        CoTaskMemFree(pfx86);
        return classic_fetch(support, services, otp, otp_cap, mid, mid_cap,
                             serial, serial_cap, udid, udid_cap, win_err);
    }

    return -10;
}
*/
import "C"

import (
	"encoding/base64"
	"errors"
	"fmt"
	"unsafe"
)

const (
	// Constants matching the values AltServer/ipatool-cpp use for the Windows
	// anisette device attestation. Routing info 17106176 is the production
	// IdMS value returned by a provisioned CoreADI. The serial/UDID are read
	// from AOSUtilities when available; these are the fallbacks AltServer uses
	// when the machine reports none.
	classicClientInfo = "<MacBookPro15,1> <Mac OS X;10.15.2;19C57> <com.apple.AuthKit/1 (com.apple.dt.Xcode/3594.4.19)>"
	classicSerial     = "C02LKHBBFD57"
	classicRouting    = "17106176"
	classicUserAgent  = "akd/1.0 CFNetwork/978.0.7 Darwin/18.7.0"
)

// fetchClassic extracts anisette data from a classic iCloud install using the
// Objective-C runtime.
func fetchClassic() (Data, error) {
	otp := make([]byte, 4096)
	mid := make([]byte, 4096)
	serial := make([]byte, 512)
	udid := make([]byte, 512)
	winErr := C.int(0)

	rc := C.ipatool_anisette_classic(
		(*C.char)(unsafe.Pointer(&otp[0])), C.int(len(otp)),
		(*C.char)(unsafe.Pointer(&mid[0])), C.int(len(mid)),
		(*C.char)(unsafe.Pointer(&serial[0])), C.int(len(serial)),
		(*C.char)(unsafe.Pointer(&udid[0])), C.int(len(udid)),
		&winErr)

	if rc != 0 {
		return Data{}, fmt.Errorf(
			"classic iCloud anisette extraction failed (code %d, windows error %d): "+
				"install iCloud for Windows from Apple's website and sign in at least once",
			int(rc), int(winErr))
	}

	otpStr := cString(otp)
	midStr := cString(mid)
	if otpStr == "" || midStr == "" {
		return Data{}, errors.New("classic iCloud anisette extraction returned empty OTP/MachineID")
	}

	serialStr := cString(serial)
	if serialStr == "" {
		serialStr = classicSerial
	}

	udidStr := cString(udid)

	// Mirror AltServer: localUserID is the base64 encoding of the machine's
	// UDID; fall back to base64 of the machine ID when AOSUtilities does not
	// expose a UDID (AltServer's non-SPOOF_MAC path).
	localUserUUID := base64.StdEncoding.EncodeToString([]byte(udidStr))
	if udidStr == "" {
		localUserUUID = base64.StdEncoding.EncodeToString([]byte(midStr))
	}

	return Data{
		OTP:           otpStr,
		MachineID:     midStr,
		LocalUserUUID: localUserUUID,
		DeviceID:      udidStr,
		ClientInfo:    classicClientInfo,
		SerialNo:      serialStr,
		RoutingInfo:   classicRouting,
		Locale:        "en_US",
		Timezone:      "PST",
		UserAgent:     classicUserAgent,
	}, nil
}
