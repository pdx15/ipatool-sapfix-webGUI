# ipatool-sapfix — cross-platform App Store IPA downloader

[![Release](https://img.shields.io/github/v/release/pdx15/ipatool-sapfix-windows?include_prereleases&label=release)](https://github.com/pdx15/ipatool-sapfix-windows/releases)
[![License](https://img.shields.io/badge/license-MIT-yellow.svg)](LICENSE)

![ipatool-sapfix App Store IPA downloader](resources/social-preview.png)

`ipatool-sapfix` is an unofficial command-line tool for searching, acquiring,
and downloading encrypted iPhone and iPad `.ipa` packages from the Apple App
Store. It is based on [`majd/ipatool`](https://github.com/majd/ipatool) and
includes a Russian/English browser GUI for Windows.

App Store login is authenticated with the SAP action signature expected by
Apple. The current implementation follows upstream `ipatool` 2.4: it runs the
original x86-64 Apple `CommerceKit`, `CommerceCore`, and `CoreFP` code locally
inside the [Unicorn](https://www.unicorn-engine.org/) emulator. It does **not**
launch `sapsigner.exe`, and it does not require iCloud, anisette, or cgo.

On first login, `ipatool` downloads the platform-specific Unicorn 2.1.4 runtime
and extracts the required Apple components from an official Apple software
update. Every downloaded archive and executable is verified against a pinned
SHA-256 digest before use and then kept in the user's cache. See
[**SAP_UNICORN.md**](SAP_UNICORN.md) for implementation and troubleshooting
details.

This is an unofficial community project. It is not affiliated with Apple or the
upstream `ipatool` maintainers.

## 🖥️ Windows GUI (Графический интерфейс)

GUI поддерживает русский и английский языки, поиск по App Store, получение
лицензии, загрузку IPA, 2FA, историю версий и перенос сессий.

1. Скачайте Windows-сборку со страницы
   [Releases](https://github.com/pdx15/ipatool-sapfix-windows/releases).
2. Запустите `ipatool.exe gui` или положите рядом скрипт `ipatool-gui.bat` и
   откройте его двойным щелчком.
3. При первом входе дождитесь подготовки SAP runtime — загрузка и проверка
   компонентов может занять несколько минут. `sapsigner.exe` и iCloud не нужны.

Подробные инструкции: [ИНСТРУКЦИЯ_GUI.md](ИНСТРУКЦИЯ_GUI.md) и
[WINDOWS_GUI_GUIDE.md](WINDOWS_GUI_GUIDE.md).

## Install

Download the archive for your operating system and architecture from the
[Releases page](https://github.com/pdx15/ipatool-sapfix-windows/releases), verify
its matching `.sha256sum`, and unpack it.

Windows PowerShell example:

```powershell
Get-FileHash .\ipatool-windows-amd64.exe -Algorithm SHA256
.\ipatool-windows-amd64.exe --version
.\ipatool-windows-amd64.exe gui
```

macOS/Linux example after unpacking:

```shell
chmod +x ./ipatool
sudo install -m 0755 ./ipatool /usr/local/bin/ipatool
ipatool --version
```

## Use

Log in interactively so the password is read from the prompt rather than from
the shell command line:

```shell
ipatool auth login --email "you@example.com"
```

Then search for, acquire, and download an app:

```shell
ipatool search "Example App"
ipatool purchase --bundle-identifier com.example.app
ipatool download --bundle-identifier com.example.app --output ExampleApp.ipa
```

Run `ipatool --help` or `ipatool <command> --help` for all available options.

### Reuse a session to skip repeated 2FA

App-specific passwords are not accepted by the App Store authentication
endpoint. A successful login, however, produces a long-lived session that can
be exported and reused on another machine or in CI:

```shell
# Log in once on any supported platform.
ipatool auth login --email "you@example.com"

# The exported JSON intentionally excludes the account password.
ipatool auth export --output account-session.json

# Import it elsewhere and use the issued App Store tokens.
ipatool auth import --input account-session.json
ipatool download --app-id 6769745089 --purchase
```

When Apple expires the token, log in again and export a fresh session.

## SAP authentication

The login sequence is:

1. Fetch Apple's current bag and validate the authentication and SAP endpoints.
2. Derive both the Store GUID and SAP hardware ID from the machine MAC address.
3. Load the checksum-verified Apple binaries and Unicorn runtime from cache (or
   download them on first use).
4. Perform the two-message SAP setup exchange with Apple.
5. Sign the exact serialized login plist and attach the binary result as
   `X-Apple-ActionSignature`.
6. Follow only validated Apple Store-pod redirects, preserving and re-signing
   the original request body.
7. Tear down the SAP session and emulator even when login fails or requests 2FA.

The SAP signer is stateful and lives only for one login attempt. Execution has a
wall-clock timeout, guest-memory bounds, checked allocations, and explicit
cleanup.

## Requirements and limitations

- Supported hosts: Windows, macOS, Linux/glibc, and Linux/musl on `amd64` or
  `arm64`.
- The first login needs internet access to the pinned Unicorn distribution,
  Apple's software-update CDN, and Apple SAP/App Store endpoints.
- Later logins use the verified user cache; no DLL or Apple framework needs to
  be placed next to `ipatool` manually.
- The App Store protocol is private and can change without notice.
- Downloaded App Store packages remain encrypted and tied to the Apple ID that
  acquired them.
- You are responsible for following Apple's terms and applicable law.

## Frequently asked questions

### Does login work on Windows without `sapsigner.exe`?

Yes. SAP signing now runs in-process through Unicorn. Remove any old
`sapsigner.exe`, `ucworker.dll`, `libunicorn.dll`, and `sap-cache` bundle placed
next to `ipatool.exe`; they are no longer read. The required, checksum-pinned
runtime files are downloaded into the user cache automatically.

### Is iCloud required on Windows?

No. The login path no longer uses GSA/SRP anisette data, so neither the classic
nor Microsoft Store edition of iCloud is required.

### Why can the first login take several minutes?

The first run downloads, extracts, and hashes Unicorn and the Apple SAP assets.
Subsequent attempts reuse the cache. If preparation was interrupted, follow the
cache reset steps in [SAP_UNICORN.md](SAP_UNICORN.md).

### Can I use an app-specific password instead of a 2FA code?

No. Use the real account password and the current verification code. Export a
successful session if you want to avoid entering them repeatedly.

### Does `ipatool` decrypt downloaded IPA files?

No. It downloads the encrypted App Store package associated with the Apple ID
that acquired it.

## Build from source

Install Go 1.25 or newer, then run:

```shell
git clone https://github.com/pdx15/ipatool-sapfix-windows.git
cd ipatool-sapfix-windows
go generate ./...
go test ./...
go build -trimpath -o ipatool .
```

Cross-compiling the Windows executable does not require MinGW or cgo:

```shell
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -o ipatool-windows-amd64.exe .
```

## Security and privacy

The repository and release artifacts contain no Apple ID, password, 2FA code,
session token, Apple framework binary, or prebuilt Unicorn DLL. Authentication
secrets are never written to verbose logs. Session exports intentionally omit
the password.

Do not publish raw authentication logs. Redact email addresses, tokens, cookies,
DSIDs, passwords, and verification codes from bug reports.

## Credits and license

Based on [`majd/ipatool`](https://github.com/majd/ipatool), including its SAP /
Unicorn implementation introduced in `v2.4.0`. Distributed under the
[MIT License](LICENSE); the original copyright and license notice are
preserved.
