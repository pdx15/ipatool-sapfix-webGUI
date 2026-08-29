# 🖥️ ipatool GUI Guide for Windows Users

The graphical user interface (GUI) for **ipatool** enables any user without technical or command-line skills to effortlessly search, license, and download iOS/iPadOS `.ipa` packages directly from the Apple App Store on Windows.

---

## 🚀 1-Click Launch

1. **Extract the archive** to any folder on your computer.
2. **Double-click:**
   - **`ipatool-gui.bat`** (or **`ipatool.exe`**).
3. The modern GUI opens automatically in your browser or desktop application window.

---

## 🌟 Key Features

1. **Apple ID Authentication with 2FA Support:**
   - Simple visual login dialog.
   - Interactive modal prompt for 6-digit two-factor verification codes.
   - HTTPS communication with Apple and local SAP signing through the Unicorn emulator.

2. **App Store Search & Discovery:**
   - Search by app name or paste an App Store URL.
   - Filter by iPhone, iPad, or Apple TV.
   - View app icons, prices, versions, and bundle IDs.
   - 1-click Download, 1-click License acquisition, and Version history.

3. **Direct Download:**
   - Download by Bundle Identifier or numeric App ID.
   - Optional version ID targeting.
   - Automatic free license acquisition (`--purchase`).

4. **Version History & Archive Downloader:**
   - Look up all historic version IDs from Apple servers.
   - Download any previously released build with one click.

5. **Download Manager & Windows Explorer Integration:**
   - Live download percentage, speed, and status.
   - "Show in Explorer" button opens the downloaded `.ipa` location immediately.

6. **Session Portability:**
   - Export session token to `account-session.json` to reuse without passwords or 2FA codes.
   - Import session token on any device.

---

## 🔑 Interactive login on Windows

Interactive App Store login (password + 2FA) works on Windows through Apple's
SAP-signed `MZFinance.woa/wa/authenticate` flow. The GUI uses the in-process SAP
implementation introduced by upstream `majd/ipatool` 2.4: original Apple
`CommerceKit`, `CommerceCore`, and `CoreFP` x86-64 code runs locally inside the
Unicorn CPU emulator.

**No separate login prerequisites are required:**

- do not install iCloud for anisette;
- do not copy `sapsigner.exe`, `ucworker.dll`, `libunicorn.dll`, or `sap-cache`
  next to `ipatool.exe`;
- `IPATOOL_SAPSIGNER` is no longer used.

On the first sign-in, keep the GUI open while it downloads and verifies the
platform-specific Unicorn runtime and the required Apple SAP assets. This can
take a few minutes. The files are stored under the user's local cache and later
sign-ins are faster. Archive and executable SHA-256 values are pinned in the
source and checked before any downloaded code is loaded.

Enter the Apple ID password, wait for SAP preparation, then enter the six-digit
verification code if the GUI requests it. The first password request and the 2FA
request each use a fresh SAP session.

If runtime preparation was interrupted, close ipatool, remove the
`%LOCALAPPDATA%\ipatool\unicorn\2.1.4` and
`%LOCALAPPDATA%\ipatool\sap\apple-assets-v2` cache directories, and retry
with a stable internet connection. See [SAP_UNICORN.md](SAP_UNICORN.md) for
technical details.

You can still export a successful session to `account-session.json` and import
it on another machine. The export contains App Store tokens, not the password.

---

## 📲 Installing Downloaded `.IPA` files on iOS

- **Sideloadly (Recommended for Windows):** Drag & drop the `.ipa` into Sideloadly, connect iPhone via USB, click Start.
- **AltStore:** Sideload wirelessly over local Wi-Fi.
- **TrollStore:** Permanent install for supported iOS versions without 7-day expiration.
