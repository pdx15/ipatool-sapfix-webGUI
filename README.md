# ipatool GUI
Download for macOS/windows - https://github.com/pdx15/ipatool-sapfix-webGUI/releases

A web-based GUI tool that searches, downloads, and installs App Store app
packages (`.ipa` files) for iOS, iPadOS, and tvOS.

![ipatool GUI preview](resources/social-preview.png)

The macOS App Store login fix adds Apple's required SAP action signature
(`X-Apple-ActionSignature`) through the macOS CommerceKit service. It also keeps
passwords and two-factor authentication codes out of verbose logs.

This is an unofficial community project. It is not affiliated with Apple or the
upstream `ipatool` maintainers.

## Features

- **App search** across the official App Store **and** a bundled catalog of apps
  that were removed from the App Store but can still be downloaded by their App
  ID (`Apps_ID_List.txt`). The two result groups are shown separately.
- **One-click download** of encrypted `.ipa` packages tied to your Apple ID.
- **Mass check & download** from a `.txt` list (`Name: AppID` format). Apps your
  account never owned are filtered out, and the remaining list can be saved back
  to a text file.
- **Version history** for downloading older builds of an app.
- **Install to device** through `libimobiledevice` (`ideviceinstaller`).
- Built-in **Apple ID login** with 2FA support, session import/export, and an
  alternative macOS login path (`Sign In Apple ID SKIP`).
- Russian and English interface, dark and light themes.

## Install

Download the release archive, extract it, and grant execute permissions:

```shell
chmod +x ipatool.2-macos-arm64
xattr -d com.apple.quarantine ipatool.2-macos-arm64
```

Run the GUI:

```shell
./ipatool.2-macos-arm64 gui
```

For the IPA installer to work, you also need `libimobiledevice`:

```shell
brew install libimobiledevice
```

If you want to install it as the `ipatool` command:

```shell
sudo install -m 0755 ipatool.2-macos-arm64 /usr/local/bin/ipatool
ipatool gui
ipatool --version
```

### Windows

A Windows build ships as `ipatool.exe`. Double-click `ipatool-gui.bat` (or run
`ipatool.exe gui`) and the interface opens in your default browser. See
[WINDOWS_GUI_GUIDE.md](WINDOWS_GUI_GUIDE.md) (English) or
[ИНСТРУКЦИЯ_GUI.md](ИНСТРУКЦИЯ_GUI.md) (Russian) for a detailed walkthrough.

## Requirements and limitations

- App Store authentication in this build requires macOS with cgo enabled.
- Release binaries are provided for Apple Silicon and Intel Macs.
- The App Store protocol is private and can change without notice.
- Downloaded App Store packages remain encrypted and are tied to the Apple ID
  that acquired them.
- Installing `.ipa` from the GUI "Install to device" tab requires
  `libimobiledevice` (`ideviceinstaller`, `idevice_id`, `idevicedeviceinfo`) on
  the machine that has the connected iOS device.
- You are responsible for following Apple's terms and applicable law.

## Frequently asked questions

### How do I fix `ipatool auth login` returning HTTP 403?

Install this macOS build and run the normal interactive login command. It signs
the authentication request with the SAP action signature now expected by the
Apple App Store endpoint.

### Does it work on Apple Silicon and Intel Macs?

The release includes native `arm64` and `amd64` binaries. Live App Store login
has been verified on Apple Silicon; reports from Intel Mac users are welcome.

### Does App Store authentication work on Windows?

Yes. On Windows, interactive login uses the legacy `MZFinance` authenticate flow
directly, signing the request with an **SAP action signature** produced by the
`sapsigner.exe` helper that ships with the tool in the `tools\` folder next to
`ipatool.exe`; its path is auto-detected or can be set via
`IPATOOL_SAPSIGNER`. (The GSA/SRP flow, which needs iCloud anisette data, is
consistently rejected by Apple on Windows with a machine-provisioning error
`-22410`, so it is not attempted there.) Enter your password and the two-factor
code directly. On macOS the SAP signature is instead generated through Apple's
CommerceKit service.

### Can I use an app-specific password instead of a 2FA code?

No. Apple's App Store authentication endpoint rejects app-specific passwords;
they only work with services such as iCloud Mail, Contacts, and Calendars. Use
the session export/import flow to avoid entering 2FA codes repeatedly.

### Does `ipatool` decrypt downloaded IPA files?

No. It downloads the encrypted App Store package associated with the Apple ID
that acquired the app.

## Build from source

Install a recent Go toolchain and the Xcode command line tools, then run:

```shell
git clone https://github.com/pdx15/ipatool-sapfix-webGUI.git
cd ipatool-sapfix-webGUI
CGO_ENABLED=1 go build -trimpath -o ipatool .
./ipatool --version
```

macOS is the primary build target. For other platforms:

- **Windows** — cross-compile with `mingw-w64` (cgo is required for the
  classic iCloud anisette path):

  ```shell
  CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
    CC=x86_64-w64-mingw32-gcc \
    go build -trimpath -o ipatool.exe .
  ```

  See `.github/workflows/build-windows.yml` and the `make` script for the
  exact environment.

- **Linux** — the code compiles with a plain `go build`, but App Store
  authentication (SAP signing) requires macOS with cgo, so the GUI works while
  live App Store login does not.

## Security and privacy

The repository and release artifacts contain no Apple ID, password, two-factor
code, session token, or local build path. GitHub secret scanning and push
protection are enabled for the public repository.

Do not publish raw authentication logs. If you report a bug, redact email
addresses, tokens, cookies, DSIDs, passwords, and two-factor codes first.

## Credits and license

Based on [`majd/ipatool`](https://github.com/majd/ipatool) and
[`maksimryabkin/ipatool-sapfix`](https://github.com/maksimryabkin/ipatool-sapfix/)
and distributed under the [MIT License](LICENSE). The original copyright and
license notice are preserved.

## Донат/Donate

Если вы хотите оставить чаевые, то можно это сделать по ссылке ниже или через QR-код:

[![Поддержать через CloudTips](resources/qrCode.png)](https://pay.cloudtips.ru/p/1569852d)

**Оставить чаевые через CloudTips:** https://pay.cloudtips.ru/p/1569852d
