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

## 🔑 Interactive login on Windows (via iCloud)

Interactive App Store login (password + 2FA) **does work on Windows** by using the **GSA
(SRP-6a) flow**, which needs Apple *anisette* headers generated from a locally installed
**iCloud** — the same mechanism Apple's own clients and tools like Signum use. The SAP
action signature is only required by an older legacy flow and is not used for login on
Windows.

**Prerequisites for Windows login:**
1. Install **iCloud** from the Microsoft Store: `https://www.microsoft.com/store/productId/9PKTQ5699M62`.
2. Open iCloud and **sign in with your Apple ID** (this provisions the `adi.pb` data the tool reads).
3. Restart the GUI and try logging in again — you will be prompted for your password and 2FA code.

If login fails with an *anisette / iCloud* error, it means iCloud is not installed or not
signed in. As a fallback, you can import a session created on a Mac:

1. **Log in once on a Mac** (Terminal): `ipatool auth login --email "you@example.com"` (enter your 2FA code if asked).
2. **Export the session:** `ipatool auth export --output account-session.json` (no password is stored in the file, only App Store tokens).
3. **Copy `account-session.json`** to the Windows PC and **import** it via **"Import Session File"** on the **"Account"** tab.

After importing, search, license acquisition, and `.IPA` downloads all work on Windows
**without a password or 2FA code**.

---

## 📲 Installing Downloaded `.IPA` files on iOS

- **Sideloadly (Recommended for Windows):** Drag & drop the `.ipa` into Sideloadly, connect iPhone via USB, click Start.
- **AltStore:** Sideload wirelessly over local Wi-Fi.
- **TrollStore:** Permanent install for supported iOS versions without 7-day expiration.
