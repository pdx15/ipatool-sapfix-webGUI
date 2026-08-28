# ipatool-sapfix — App Store IPA downloader (macOS, Windows, Linux)

[![Release](https://img.shields.io/github/v/release/maksimryabkin/ipatool-sapfix?include_prereleases&label=release)](https://github.com/maksimryabkin/ipatool-sapfix/releases)
[![License](https://img.shields.io/badge/license-MIT-yellow.svg)](LICENSE)

![ipatool-sapfix macOS App Store HTTP 403 fix and IPA downloader](resources/social-preview.png)

`ipatool-sapfix` is an unofficial command-line tool for macOS, Windows, and
Linux for searching, acquiring, and downloading encrypted iPhone and iPad
`.ipa` packages from the Apple App Store. This standalone build is based on
[`ipatool`](https://github.com/majd/ipatool) and restores `ipatool auth login`
when Apple authentication fails with:

```text
request failed: unexpected response from Apple (HTTP 403): empty or non-plist body
```

The macOS App Store login fix adds Apple's required SAP action signature
(`X-Apple-ActionSignature`) through the macOS CommerceKit service. It also keeps
passwords and two-factor authentication codes out of verbose logs.

This is an unofficial community project. It is not affiliated with Apple or the
upstream `ipatool` maintainers.

## Download

Download the current prerelease from the
[Releases page](https://github.com/maksimryabkin/ipatool-sapfix/releases/tag/2.3.2-sapfix.1).

| Platform | Architecture | Archive |
| --- | --- | --- |
| macOS Apple Silicon | `arm64` | `ipatool-2.3.2-sapfix.1-macos-arm64.tar.gz` |
| macOS Intel | `x86_64` | `ipatool-2.3.2-sapfix.1-macos-amd64.tar.gz` |
| Windows (x64) | `amd64` | `ipatool-<version>-windows-amd64.tar.gz` |
| Windows (ARM64) | `arm64` | `ipatool-<version>-windows-arm64.tar.gz` |
| Linux (x64) | `amd64` | `ipatool-<version>-linux-amd64.tar.gz` |
| Linux (ARM64) | `arm64` | `ipatool-<version>-linux-arm64.tar.gz` |

Each archive has a matching `.sha256sum` file attached to the release. Note
that releases tagged before Windows and Linux builds were added only contain
the macOS binaries; build from source until the next release.

## Install

Apple Silicon example:

```shell
shasum -a 256 -c ipatool-2.3.2-sapfix.1-macos-arm64.tar.gz.sha256sum
tar -xzf ipatool-2.3.2-sapfix.1-macos-arm64.tar.gz
sudo install -m 0755 \
  bin/ipatool-2.3.2-sapfix.1-macos-arm64 \
  /usr/local/bin/ipatool
ipatool --version
```

For an Intel Mac, replace `arm64` with `amd64` in the archive and binary names.

If macOS reports that the downloaded binary cannot be opened, verify the
checksum first and then remove only its quarantine attribute:

```shell
xattr -d com.apple.quarantine bin/ipatool-2.3.2-sapfix.1-macos-arm64
```

### Windows

`tar` is built into Windows 10 and later. Unpack the archive and run the
binary, for example from PowerShell:

```powershell
tar -xzf ipatool-<version>-windows-amd64.tar.gz
.\bin\ipatool-<version>-windows-amd64.exe --version
```

`ipatool auth login` is not available on Windows, so transfer a session from
a Mac first (see [Reuse a session](#reuse-a-session-to-skip-two-factor-authentication-ci)):

```shell
# on your Mac
ipatool auth export --output account-session.json
```

```powershell
# on Windows
.\bin\ipatool-<version>-windows-amd64.exe auth import --input .\account-session.json
```

The session is stored in the Windows Credential Manager. After that, all the
normal commands (`search`, `purchase`, `download`, ...) work on Windows.

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
ipatool download --bundle-identifier com.example.app \
  --output ExampleApp.ipa
```

Run `ipatool --help` or `ipatool <command> --help` for all available options.

### Reuse a session to skip two-factor authentication (CI)

App-specific passwords are not accepted by the App Store authentication
endpoint, so `ipatool` needs the real account password and, for two-factor
accounts, a fresh 2FA code to sign in. A successful login, however, produces
a long-lived session that can be exported and reused elsewhere, e.g. in
GitHub Actions:

1. Log in once interactively on your Mac.

   ```shell
   ipatool auth login --email "you@example.com"
   ```

2. Export the active session. The account password is never included; only
   the tokens issued by the App Store are exported.

   ```shell
   ipatool auth export --output account-session.json
   ```

3. Import the session on the other machine or CI runner, then download.

   ```shell
   ipatool auth import --input account-session.json
   ipatool download --app-id 6769745089 --purchase
   ```

When Apple eventually expires the token, `ipatool download` reports that the
password token is expired; repeat the steps above to export a fresh session.

## Requirements and limitations

- `ipatool auth login` requires macOS with cgo enabled, because Apple's SAP
  action signature can only be produced through the macOS CommerceKit service.
  On Windows and Linux, use a session exported from a Mac (`auth export` /
  `auth import`) or the `IPATOOL_SESSION` environment variable instead.
- All other commands (`search`, `purchase`, `download`, `list-versions`,
  `get-version-metadata`) work on macOS, Windows, and Linux.
- On Windows the session is stored in the Windows Credential Manager; on
  macOS in the Keychain; on Linux in the Secret Service (GNOME Keyring or
  KWallet) when available, otherwise in the encrypted file under
  `~/.ipatool/`.
- Release binaries are provided for macOS, Windows, and Linux (both `amd64`
  and `arm64` where supported).
- The App Store protocol is private and can change without notice.
- Downloaded App Store packages remain encrypted and are tied to the Apple ID
  that acquired them.
- You are responsible for following Apple's terms and applicable law.

## Frequently asked questions

### How do I fix `ipatool auth login` returning HTTP 403?

Install this macOS build and run the normal interactive login command. It signs
the authentication request with the SAP action signature now expected by the
Apple App Store endpoint.

### Does it work on Apple Silicon and Intel Macs?

The release includes native `arm64` and `amd64` binaries. Live App Store login
has been verified on Apple Silicon; reports from Intel Mac users are welcome.

### Does it work on Windows?

Yes. Run the Windows binary and transfer a session from a Mac: log in once on
macOS, then run `ipatool auth export --output account-session.json` there and
`ipatool auth import --input account-session.json` on Windows. After that,
`search`, `purchase`, `download`, `list-versions`, and `get-version-metadata`
all work on Windows. Only `ipatool auth login` is macOS-only, because Apple's
`X-Apple-ActionSignature` is generated through Apple's CommerceKit service,
which is available on macOS only.

### Why is App Store authentication macOS-only?

The required `X-Apple-ActionSignature` is generated through Apple's CommerceKit
service, which is available on macOS. Other platforms cannot use this signing
implementation; use the session export/import flow described above.

### Can I use an app-specific password instead of a 2FA code?

No. Apple's App Store authentication endpoint rejects app-specific passwords;
they only work with services such as iCloud Mail, Contacts, and Calendars.
Use the session export/import flow described above to avoid entering 2FA
codes repeatedly.

### Does `ipatool` decrypt downloaded IPA files?

No. It downloads the encrypted App Store package associated with the Apple ID
that acquired the app.

## Support

Use [GitHub Issues](https://github.com/maksimryabkin/ipatool-sapfix/issues) for
bugs and compatibility reports. Include the macOS version, Mac architecture,
`ipatool --version`, and a redacted error message. Never include credentials or
raw authentication data.

## Build from source

Install a recent Go toolchain and the Xcode command line tools, then run:

```shell
git clone https://github.com/maksimryabkin/ipatool-sapfix.git
cd ipatool-sapfix
CGO_ENABLED=1 go build -trimpath -o ipatool .
./ipatool --version
```

To build for another platform (e.g. a Windows binary from any host), set
`GOOS`/`GOARCH` instead. cgo (and therefore the macOS signing service) is
only used when building for macOS:

```shell
GOOS=windows GOARCH=amd64 go build -trimpath -o ipatool.exe .
GOOS=linux GOARCH=amd64 go build -trimpath -o ipatool-linux-amd64 .
```

## Security and privacy

The repository and release artifacts contain no Apple ID, password, two-factor
code, session token, or local build path. GitHub secret scanning and push
protection are enabled for the public repository.

Do not publish raw authentication logs. If you report a bug, redact email
addresses, tokens, cookies, DSIDs, passwords, and two-factor codes first.

## Credits and license

Based on [`majd/ipatool`](https://github.com/majd/ipatool) and distributed under
the [MIT License](LICENSE). The original copyright and license notice are
preserved.
