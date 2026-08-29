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
   - Safe and direct communication with Apple servers over SRP-6a / GSA protocol.

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

## 📲 Installing Downloaded `.IPA` files on iOS

- **Sideloadly (Recommended for Windows):** Drag & drop the `.ipa` into Sideloadly, connect iPhone via USB, click Start.
- **AltStore:** Sideload wirelessly over local Wi-Fi.
- **TrollStore:** Permanent install for supported iOS versions without 7-day expiration.
