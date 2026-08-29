# ipatool-sapfix — macOS App Store IPA downloader

[![Release](https://img.shields.io/github/v/release/maksimryabkin/ipatool-sapfix?include_prereleases&label=release)](https://github.com/maksimryabkin/ipatool-sapfix/releases)
[![License](https://img.shields.io/badge/license-MIT-yellow.svg)](LICENSE)

![ipatool-sapfix macOS App Store HTTP 403 fix and IPA downloader](resources/social-preview.png)

`ipatool-sapfix` is an unofficial macOS command-line tool for searching,
acquiring, and downloading encrypted iPhone and iPad `.ipa` packages from the
Apple App Store. This standalone build is based on
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

## 🖥️ Windows GUI (Графический интерфейс для Windows)

Для удобства рядовых пользователей добавлен полноценный **графический интерфейс (GUI)** с поддержкой русского языка, поиском по App Store, загрузкой по 1 клику, управлением 2FA кодами и историей версий.

### Запуск GUI на Windows (1 клик):
- **Дважды кликните по файлу `ipatool-gui.bat`** (или запустите `ipatool.exe gui`).
- Автоматически откроется удобный интерфейс с карточками приложений, прогресс-баром скачивания и интеграцией с Проводником Windows.
- Подробное руководство пользователя на русском языке доступно в файле: **[ИНСТРУКЦИЯ_GUI.md](ИНСТРУКЦИЯ_GUI.md)** (English: **[WINDOWS_GUI_GUIDE.md](WINDOWS_GUI_GUIDE.md)**).

> 🔑 **Интерактивный вход (пароль + 2FA) в GUI на Windows** выполняется через **legacy-путь** (`MZFinance.woa/wa/authenticate`), который подписывает запрос **SAP-подписью** через `sapsigner.exe` из **Signum** (altstore.io) — его путь определяется автоматически или через переменную `IPATOOL_SAPSIGNER`. (Протокол GSA/SRP с анизетом из iCloud на Windows стабильно отклоняется Apple ошибкой `-22410`, поэтому на Windows он не используется.) **Установите Signum**, затем повторите вход. Если Signum недоступен, всегда можно **импортировать сессию** с Mac (вкладка «Аккаунт» → «Импорт файла сессии»).

> ℹ️ **Откуда берётся `sapsigner.exe`.** Этот файл — готовый бинарник, который поставляется вместе с **Signum** (Windows-компаньон AltStore) и находится в `%LOCALAPPDATA%\Signum\resources\apple-tools\windows-x64\v3-legacy\sapsigner.exe`. Он выполняет SAP-подпись, эмулируя проприетарные фреймворки Apple (`CommerceKit`, `CoreFP` из соседней папки `sap-cache`). Наша программа просто запускает уже установленный у пользователя `sapsigner.exe` как внешний процесс и находит его автоматически (или по `IPATOOL_SAPSIGNER`). В **публичном** репозитории этот бинарник не распространяется (чужой проприетарный код), но владелец **приватного** репозитория может положить его рядом с `ipatool.exe` или в `tools\`, чтобы Signum не требовался — см. **[WINDOWS_SAPSIGNER_BUNDLE.md](WINDOWS_SAPSIGNER_BUNDLE.md)**.

---

## Download

Download the current prerelease from the
[Releases page](https://github.com/maksimryabkin/ipatool-sapfix/releases/tag/2.3.2-sapfix.1).

| Mac | `uname -m` | Archive |
| --- | --- | --- |
| Apple Silicon | `arm64` | `ipatool-2.3.2-sapfix.1-macos-arm64.tar.gz` |
| Intel | `x86_64` | `ipatool-2.3.2-sapfix.1-macos-amd64.tar.gz` |

Each archive has a matching `.sha256sum` file attached to the release.

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

- App Store authentication in this build requires macOS with cgo enabled.
- Release binaries are provided for Apple Silicon and Intel Macs.
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

### Does App Store authentication work on Windows?

Yes. On Windows, interactive login uses the legacy `MZFinance` authenticate flow
directly, signing the request with an **SAP action signature** produced by the
`sapsigner.exe` helper bundled with **Signum** (altstore.io); its path is
auto-detected or can be set via `IPATOOL_SAPSIGNER`. (The GSA/SRP flow, which
needs iCloud anisette data, is consistently rejected by Apple on Windows with a
machine-provisioning error `-22410`, so it is not attempted there.) Install
Signum and log in; two-factor codes are entered directly. On macOS the SAP
signature is instead generated through Apple's CommerceKit service.

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
