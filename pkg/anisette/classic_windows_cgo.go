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

// classic_fetch extracts OTP + machine ID from classic iCloud installed under
// Common Files\Apple. supportDir holds objc.dll/Foundation.dll; servicesDir
// holds AOSKit.dll. Returns 0 on success.
static int classic_fetch(const wchar_t* support_dir, const wchar_t* services_dir,
                         char* otp, int otp_cap, char* mid, int mid_cap) {
    wchar_t objc_path[MAX_PATH];
    wchar_t foundation_path[MAX_PATH];
    wchar_t aoskit_path[MAX_PATH];

    swprintf(objc_path, MAX_PATH, L"%ls\\objc.dll", support_dir);
    swprintf(foundation_path, MAX_PATH, L"%ls\\Foundation.dll", support_dir);
    swprintf(aoskit_path, MAX_PATH, L"%ls\\AOSKit.dll", services_dir);

    HMODULE objc_lib = LoadLibraryW(objc_path);
    if (!objc_lib) return -1;
    HMODULE foundation_lib = LoadLibraryW(foundation_path);
    HMODULE aoskit_lib = LoadLibraryW(aoskit_path);
    if (!foundation_lib || !aoskit_lib) return -2;

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
int ipatool_anisette_classic(char* otp, int otp_cap, char* mid, int mid_cap) {
    wchar_t support[MAX_PATH];
    wchar_t services[MAX_PATH];

    wchar_t* pf = NULL;
    wchar_t* pfx86 = NULL;
    SHGetKnownFolderPath(&kFolderIdProgramFilesCommon, 0, NULL, &pf);
    SHGetKnownFolderPath(&kFolderIdProgramFilesCommonX86, 0, NULL, &pfx86);

    if (pf) {
        swprintf(support, MAX_PATH, L"%ls\\Apple\\Apple Application Support", pf);
        swprintf(services, MAX_PATH, L"%ls\\Apple\\Internet Services", pf);
        CoTaskMemFree(pf);
        if (classic_fetch(support, services, otp, otp_cap, mid, mid_cap) == 0) {
            if (pfx86) CoTaskMemFree(pfx86);
            return 0;
        }
    }

    if (pfx86) {
        swprintf(support, MAX_PATH, L"%ls\\Apple\\Apple Application Support", pfx86);
        swprintf(services, MAX_PATH, L"%ls\\Apple\\Internet Services", pfx86);
        CoTaskMemFree(pfx86);
        return classic_fetch(support, services, otp, otp_cap, mid, mid_cap);
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
	// anisette device attestation.
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

	rc := C.ipatool_anisette_classic(
		(*C.char)(unsafe.Pointer(&otp[0])), C.int(len(otp)),
		(*C.char)(unsafe.Pointer(&mid[0])), C.int(len(mid)))

	if rc != 0 {
		return Data{}, fmt.Errorf(
			"classic iCloud anisette extraction failed (code %d): "+
				"install iCloud for Windows from Apple's website and sign in at least once", int(rc))
	}

	otpStr := cString(otp)
	midStr := cString(mid)
	if otpStr == "" || midStr == "" {
		return Data{}, errors.New("classic iCloud anisette extraction returned empty OTP/MachineID")
	}

	// Local user UUID is the base64-encoded machine identifier, mirroring
	// AltServer's non-SPOOF_MAC path.
	localUserUUID := base64.StdEncoding.EncodeToString([]byte(midStr))

	return Data{
		OTP:           otpStr,
		MachineID:     midStr,
		LocalUserUUID: localUserUUID,
		ClientInfo:    classicClientInfo,
		SerialNo:      classicSerial,
		RoutingInfo:   classicRouting,
		Locale:        "en_US",
		Timezone:      "PST",
		UserAgent:     classicUserAgent,
	}, nil
}
