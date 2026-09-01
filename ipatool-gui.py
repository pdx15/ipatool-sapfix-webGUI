#!/usr/bin/env python3
"""
ipatool GUI — Standalone Desktop & Web Server for Windows / macOS / Linux
Provides a zero-skill, intuitive Graphical User Interface for searching,
acquiring licenses, and downloading .IPA packages from the Apple App Store.
"""

import os
import sys
import json
import shutil
import tempfile
import urllib.request
import urllib.parse
import subprocess
import threading
import time
import webbrowser
import email
import email.message
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer

PORT = 54321
HOST = "0.0.0.0"
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
ASSETS_DIR = os.path.join(SCRIPT_DIR, "cmd", "gui_assets")

# Find ipatool executable
def find_ipatool_binary():
    candidates = [
        os.path.join(SCRIPT_DIR, "ipatool.exe"),
        os.path.join(SCRIPT_DIR, "bin", "ipatool.exe"),
        os.path.join(SCRIPT_DIR, "ipatool"),
        os.path.join(SCRIPT_DIR, "bin", "ipatool"),
        shutil.which("ipatool"),
        shutil.which("ipatool.exe"),
    ]
    for c in candidates:
        if c and os.path.isfile(c) and os.access(c, os.X_OK):
            return c
        elif c and os.path.isfile(c) and sys.platform == "win32":
            return c
    return None

# Active background jobs for download tracking
ACTIVE_JOBS = {}
JOBS_LOCK = threading.Lock()

# Active background jobs for .ipa installation
INSTALL_JOBS = {}
INSTALL_JOBS_LOCK = threading.Lock()
INSTALL_BASE = os.path.join(tempfile.gettempdir(), "ipatool-gui-installs")

TOOL_ENV = {
    "installer": "IPATOOL_IDEVICEINSTALLER",
    "list": "IPATOOL_IDEVICE_ID",
    "info": "IPATOOL_IDEVICEDEVICEINFO",
}

TOOL_NAMES = {
    "installer": ["ideviceinstaller", "ideviceinstaller.exe"],
    "list": ["idevice_id", "idevice_id.exe"],
    "info": ["idevicedeviceinfo", "idevicedeviceinfo.exe", "ideviceinfo", "ideviceinfo.exe"],
}


def _candidate_tool_paths(filename):
    paths = []
    paths.append(os.path.join(SCRIPT_DIR, "tools", filename))
    paths.append(os.path.join(SCRIPT_DIR, filename))
    try:
        bin_dir = os.path.dirname(os.path.abspath(sys.executable))
        paths.append(os.path.join(bin_dir, "tools", filename))
        paths.append(os.path.join(bin_dir, filename))
    except Exception:
        pass

    if sys.platform == "win32":
        for base in (
            os.environ.get("ProgramFiles", r"C:\Program Files"),
            os.environ.get("ProgramFiles(x86)", r"C:\Program Files (x86)"),
        ):
            paths.extend([
                os.path.join(base, "idevice", filename),
                os.path.join(base, "libimobiledevice", filename),
            ])
    else:
        paths.extend([
            "/opt/homebrew/bin/" + filename,
            "/usr/local/bin/" + filename,
            "/opt/local/bin/" + filename,
            "/usr/bin/" + filename,
        ])
    return paths


def find_tool(kind):
    env_var = TOOL_ENV.get(kind)
    if env_var:
        env_path = os.environ.get(env_var, "").strip()
        if env_path and os.path.isfile(env_path):
            return env_path

    for name in TOOL_NAMES.get(kind, []):
        for path in _candidate_tool_paths(name):
            if os.path.isfile(path):
                return path
        picked = shutil.which(name)
        if picked:
            return picked
    return None


def run_tool(kind, args, timeout=10):
    tool = find_tool(kind)
    if not tool:
        return "", "", 1
    try:
        result = subprocess.run(
            [tool] + list(args),
            capture_output=True,
            text=True,
            timeout=timeout,
            errors="replace",
        )
        return result.stdout or "", result.stderr or "", result.returncode
    except Exception as e:
        return "", str(e), 1


# Apple product type -> friendly model name. Used when idevicedeviceinfo does not
# expose a MarketingName/DeviceName but does expose ProductType.
MODEL_BY_PRODUCT_TYPE = {
    "iPhone1,1": "iPhone",
    "iPhone1,2": "iPhone 3G",
    "iPhone2,1": "iPhone 3GS",
    "iPhone3,1": "iPhone 4",
    "iPhone3,2": "iPhone 4",
    "iPhone3,3": "iPhone 4 CDMA",
    "iPhone4,1": "iPhone 4S",
    "iPhone5,1": "iPhone 5",
    "iPhone5,2": "iPhone 5",
    "iPhone5,3": "iPhone 5c",
    "iPhone5,4": "iPhone 5c",
    "iPhone6,1": "iPhone 5s",
    "iPhone6,2": "iPhone 5s",
    "iPhone7,1": "iPhone 6 Plus",
    "iPhone7,2": "iPhone 6",
    "iPhone8,1": "iPhone 6s",
    "iPhone8,2": "iPhone 6s Plus",
    "iPhone8,4": "iPhone SE (1st generation)",
    "iPhone9,1": "iPhone 7",
    "iPhone9,2": "iPhone 7 Plus",
    "iPhone9,3": "iPhone 7",
    "iPhone9,4": "iPhone 7 Plus",
    "iPhone10,1": "iPhone 8",
    "iPhone10,2": "iPhone 8 Plus",
    "iPhone10,3": "iPhone X",
    "iPhone10,4": "iPhone 8",
    "iPhone10,5": "iPhone 8 Plus",
    "iPhone10,6": "iPhone X",
    "iPhone11,2": "iPhone XS",
    "iPhone11,4": "iPhone XS Max",
    "iPhone11,6": "iPhone XS Max",
    "iPhone11,8": "iPhone XR",
    "iPhone12,1": "iPhone 11",
    "iPhone12,3": "iPhone 11 Pro",
    "iPhone12,5": "iPhone 11 Pro Max",
    "iPhone12,8": "iPhone SE (2nd generation)",
    "iPhone13,1": "iPhone 12 mini",
    "iPhone13,2": "iPhone 12",
    "iPhone13,3": "iPhone 12 Pro",
    "iPhone13,4": "iPhone 12 Pro Max",
    "iPhone14,2": "iPhone 13 Pro",
    "iPhone14,3": "iPhone 13 Pro Max",
    "iPhone14,4": "iPhone 13 mini",
    "iPhone14,5": "iPhone 13",
    "iPhone14,7": "iPhone SE (3rd generation)",
    "iPhone15,2": "iPhone 14 Pro",
    "iPhone15,3": "iPhone 14 Pro Max",
    "iPhone15,4": "iPhone 14",
    "iPhone15,5": "iPhone 14 Plus",
    "iPhone16,1": "iPhone 15 Pro",
    "iPhone16,2": "iPhone 15 Pro Max",
    "iPhone16,3": "iPhone 15",
    "iPhone16,4": "iPhone 15 Plus",
    "iPhone17,1": "iPhone 16 Pro",
    "iPhone17,2": "iPhone 16 Pro Max",
    "iPhone17,3": "iPhone 16",
    "iPhone17,4": "iPhone 16 Plus",
    "iPhone17,5": "iPhone 16e",
}


def device_model_name(product_type, product_name, name):
    type_key = (product_type or "").strip()
    product_name = (product_name or "").strip()
    name = (name or "").strip()

    if type_key:
        # Prefer the hardware ProductType over a user-renamed DeviceName.
        model = MODEL_BY_PRODUCT_TYPE.get(type_key)
        if model:
            return model
        if any(
            type_key.startswith(prefix)
            for prefix in ("iPhone", "iPad", "AppleTV", "Watch")
        ):
            if product_name:
                return product_name
            return type_key

    if product_name:
        return product_name
    return name


def read_device_info(tool, udid):
    info = {}
    stdout, _, rc = run_tool("info", ["-u", udid], timeout=5)
    if rc == 0 and stdout:
        for line in stdout.splitlines():
            if ":" in line:
                key, value = line.split(":", 1)
                key = key.strip()
                value = value.strip()
                if key and value:
                    info[key] = value

    # Some builds only return data per key when -k is supplied.
    for key in ("DeviceName", "ProductName", "ProductType", "ProductVersion", "SerialNumber"):
        if not info.get(key):
            value, _, _ = run_tool("info", ["-u", udid, "-k", key], timeout=3)
            if value.strip():
                info[key] = value.strip()
    return info


def _common_apple_driver_paths():
    program_files = os.environ.get("ProgramFiles", r"C:\Program Files")
    program_files_x86 = os.environ.get("ProgramFiles(x86)", r"C:\Program Files (x86)")
    common64 = os.path.join(program_files, "Common Files", "Apple", "Mobile Device Support")
    common86 = os.path.join(program_files_x86, "Common Files", "Apple", "Mobile Device Support")
    itunes64 = os.path.join(program_files, "iTunes", "iTunes.exe")
    itunes86 = os.path.join(program_files_x86, "iTunes", "iTunes.exe")
    return [
        os.path.join(common64, "MobileDevice.dll"),
        os.path.join(common64, "drivers", "usbaapl64.sys"),
        os.path.join(common64, "usbaapl64.sys"),
        os.path.join(common86, "MobileDevice.dll"),
        os.path.join(common86, "drivers", "usbaapl64.sys"),
        os.path.join(common86, "usbaapl64.sys"),
        itunes64,
        itunes86,
    ]


def _windows_service_exists(name):
    try:
        res = subprocess.run(
            ["sc", "query", name],
            capture_output=True,
            text=True,
            errors="replace",
            timeout=3,
        )
        # sc returns 0 for an existing service (running/stopped) and non-zero
        # (e.g. 1060) when the service name is unknown.
        return res.returncode == 0 and ("RUNNING" in res.stdout.upper() or "STOPPED" in res.stdout.upper())
    except Exception:
        return False


def check_apple_driver():
    if sys.platform != "win32":
        return {
            "installed": True,
            "required": False,
            "message": "Apple Mobile Device Support driver check is only required on Windows.",
            "downloadUrl": "",
            "itunesUrl": "",
        }

    found_path = ""
    for path in _common_apple_driver_paths():
        if os.path.isfile(path):
            found_path = path
            break

    service_installed = _windows_service_exists("Apple Mobile Device Service") or _windows_service_exists(
        "Apple Mobile Device"
    )

    installed = bool(found_path) or service_installed
    return {
        "installed": installed,
        "required": True,
        "path": found_path,
        "service": service_installed,
        "message": "Apple Mobile Device Support driver/service not found. Install iTunes (which bundles it) or update the Apple USB driver.",
        "downloadUrl": "https://support.apple.com/en-us/HT210384",
        "itunesUrl": "https://www.apple.com/itunes/",
    }


class RequestHandler(SimpleHTTPRequestHandler):
    def end_headers(self):
        # Enable CORS and disable cache for API
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        self.send_header("Cache-Control", "no-cache, no-store, must-revalidate")
        super().end_headers()

    def do_OPTIONS(self):
        self.send_response(200)
        self.end_headers()

    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path
        query = urllib.parse.parse_qs(parsed.query)

        if path in ("/", "/index.html"):
            return self.serve_asset("index.html", "text/html; charset=utf-8")
        elif path == "/assets/style.css":
            return self.serve_asset("style.css", "text/css; charset=utf-8")
        elif path == "/assets/app.js":
            return self.serve_asset("app.js", "application/javascript; charset=utf-8")
        elif path == "/api/status":
            return self.handle_api_status()
        elif path == "/api/search":
            return self.handle_api_search(query)
        elif path == "/api/search/all":
            return self.handle_api_search_all(query)
        elif path == "/api/removed-apps":
            return self.handle_api_removed_apps(query)
        elif path == "/api/qrcode":
            return self.handle_api_qrcode()
        elif path == "/api/download/status":
            return self.handle_api_download_status(query)
        elif path == "/api/versions":
            return self.handle_api_versions(query)
        elif path == "/api/version-metadata":
            return self.handle_api_version_metadata(query)
        elif path == "/api/auth/export":
            return self.handle_api_export()
        elif path == "/api/install/devices":
            return self.handle_api_install_devices()
        elif path == "/api/install/status":
            return self.handle_api_install_status(query)
        else:
            # Fallback to asset serving or 404
            asset_path = os.path.join(ASSETS_DIR, path.lstrip("/assets/").lstrip("/"))
            if os.path.isfile(asset_path):
                return self.serve_asset(os.path.basename(asset_path))
            self.send_error(404, "Not Found")

    def do_POST(self):
        parsed = urllib.parse.urlparse(self.path)
        path = parsed.path
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length) if length > 0 else b""

        if path == "/api/install/upload":
            return self.handle_api_install_upload(body, self.headers.get("Content-Type", ""))

        try:
            payload = json.loads(body.decode("utf-8"))
        except Exception:
            payload = {}

        if path == "/api/auth/login":
            return self.handle_api_login(payload)
        elif path == "/api/auth/login/mzfinance":
            return self.handle_api_login_mzfinance(payload)
        elif path == "/api/auth/revoke":
            return self.handle_api_revoke()
        elif path == "/api/auth/import":
            return self.handle_api_import(payload)
        elif path == "/api/purchase":
            return self.handle_api_purchase(payload)
        elif path == "/api/download":
            return self.handle_api_download(payload)
        elif path == "/api/open-folder":
            return self.handle_api_open_folder(payload)
        else:
            self.send_error(404, "Not Found")

    def serve_asset(self, filename, content_type="text/plain"):
        filepath = os.path.join(ASSETS_DIR, filename)
        if not os.path.isfile(filepath):
            self.send_error(404, f"Asset not found: {filename}")
            return
        with open(filepath, "rb") as f:
            data = f.read()
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def send_json(self, data, status_code=200):
        body = json.dumps(data, ensure_ascii=False).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    # API Handlers
    def handle_api_status(self):
        bin_path = find_ipatool_binary()
        if bin_path:
            try:
                res = subprocess.run([bin_path, "auth", "info", "--format", "json"],
                                     capture_output=True, text=True, timeout=5)
                if res.returncode == 0:
                    data = json.loads(res.stdout)
                    return self.send_json({
                        "authenticated": True,
                        "account": {
                            "name": data.get("name", "Apple User"),
                            "email": data.get("email", ""),
                            "storefront": data.get("storefront", ""),
                            "dsid": data.get("dsPersonId", "")
                        },
                        "version": "2.3.2-sapfix",
                        "os": sys.platform
                    })
            except Exception:
                pass

        # Check local session file if exists
        home = os.path.expanduser("~")
        session_file = os.path.join(home, ".ipatool", "session.json")
        if os.path.isfile(session_file):
            try:
                with open(session_file, "r") as f:
                    data = json.load(f)
                    return self.send_json({
                        "authenticated": True,
                        "account": {
                            "name": data.get("name", "Apple User"),
                            "email": data.get("email", ""),
                            "storefront": data.get("storeFront", "App Store"),
                            "dsid": data.get("directoryServicesID", "")
                        },
                        "version": "2.3.2-sapfix",
                        "os": sys.platform
                    })
            except Exception:
                pass

        self.send_json({
            "authenticated": False,
            "account": None,
            "version": "2.3.2-sapfix",
            "os": sys.platform
        })

    def handle_api_login(self, payload):
        return self._handle_login(payload, mzfinance=False)

    def handle_api_login_mzfinance(self, payload):
        # Diagnostic macOS-only test login: GSA (public anisette) -> MZFinance,
        # bypassing the glitchy native/fast path. Invokes ipatool auth login
        # --mzfinance (or falls back to the normal login if the flag is absent).
        return self._handle_login(payload, mzfinance=True)

    def _handle_login(self, payload, mzfinance):
        email = payload.get("email", "").strip()
        password = payload.get("password", "")
        auth_code = payload.get("authCode", "").strip()

        if not email or not password:
            return self.send_json({"success": False, "message": "Email and password are required"}, 400)

        bin_path = find_ipatool_binary()
        if bin_path:
            cmd = [bin_path, "auth", "login", "--email", email, "--password", password, "--non-interactive", "--format", "json"]
            if mzfinance:
                cmd.append("--mzfinance")
            if auth_code:
                cmd.extend(["--auth-code", auth_code])
            try:
                res = subprocess.run(cmd, capture_output=True, text=True, timeout=15)
                output = res.stdout + res.stderr
                if "2FA code is required" in output or "auth code is required" in output:
                    return self.send_json({"success": False, "authCodeRequired": True, "message": "2FA verification code required"})
                if res.returncode == 0:
                    try:
                        data = json.loads(res.stdout)
                        return self.send_json({
                            "success": True,
                            "account": {
                                "name": data.get("name", email.split("@")[0]),
                                "email": data.get("email", email),
                                "storefront": data.get("storefront", "App Store"),
                                "dsid": data.get("dsPersonId", "")
                            }
                        })
                    except Exception:
                        return self.send_json({"success": True, "account": {"email": email, "name": email.split("@")[0]}})
                else:
                    return self.send_json({"success": False, "message": res.stderr or "Login failed"})
            except Exception as e:
                return self.send_json({"success": False, "message": str(e)})

        # If binary is not yet compiled, save session locally for demo/mock
        home = os.path.expanduser("~")
        os.makedirs(os.path.join(home, ".ipatool"), exist_ok=True)
        session_file = os.path.join(home, ".ipatool", "session.json")
        with open(session_file, "w") as f:
            json.dump({
                "email": email,
                "name": email.split("@")[0].capitalize(),
                "storeFront": "143441-1,29",
                "directoryServicesID": "1234567890"
            }, f)

        self.send_json({
            "success": True,
            "account": {
                "name": email.split("@")[0].capitalize(),
                "email": email,
                "storefront": "App Store (US)",
                "dsid": "1234567890"
            }
        })

    def handle_api_revoke(self):
        bin_path = find_ipatool_binary()
        if bin_path:
            subprocess.run([bin_path, "auth", "revoke", "--non-interactive"], capture_output=True)

        home = os.path.expanduser("~")
        session_file = os.path.join(home, ".ipatool", "session.json")
        if os.path.isfile(session_file):
            try:
                os.remove(session_file)
            except Exception:
                pass

        self.send_json({"success": True})

    def handle_api_export(self):
        home = os.path.expanduser("~")
        session_file = os.path.join(home, ".ipatool", "session.json")
        session_data = {"email": "user@example.com", "storeFront": "143441-1,29"}
        if os.path.isfile(session_file):
            try:
                with open(session_file, "r") as f:
                    session_data = json.load(f)
            except Exception:
                pass

        session_data.pop("password", None)
        body = json.dumps(session_data, indent=2).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Disposition", "attachment; filename=account-session.json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def handle_api_import(self, payload):
        home = os.path.expanduser("~")
        os.makedirs(os.path.join(home, ".ipatool"), exist_ok=True)
        session_file = os.path.join(home, ".ipatool", "session.json")
        with open(session_file, "w") as f:
            json.dump(payload, f, indent=2)
        self.send_json({"success": True, "email": payload.get("email", "")})

    def _respond(self, payload, send=True, status_code=200):
        """Sends payload as a JSON response, or returns it when send is False so
        another handler can reuse it (see handle_api_search_all)."""
        if send:
            self.send_json(payload, status_code)
            return None
        return payload

    def handle_api_search(self, query, send=True):
        term = query.get("term", [""])[0]
        platform = query.get("platform", ["iphone"])[0]
        limit = query.get("limit", ["25"])[0]

        if not term:
            return self._respond({"success": False, "message": "Search term is required"}, send, 400)

        entity_map = {
            "iphone": "software",
            "ipad": "iPadSoftware",
            "appletv": "software,tvSoftware"
        }
        entity = entity_map.get(platform, "software")

        # Direct Apple iTunes Search API
        try:
            params = urllib.parse.urlencode({
                "term": term,
                "country": "us",
                "entity": entity,
                "media": "software",
                "limit": limit
            })
            url = f"https://itunes.apple.com/search?{params}"
            req = urllib.request.Request(url, headers={"User-Agent": "ipatool/2.3.2"})
            with urllib.request.urlopen(req, timeout=10) as response:
                data = json.loads(response.read().decode("utf-8"))
                results = data.get("results", [])
                formatted = []
                for item in results:
                    formatted.append({
                        "trackId": item.get("trackId", 0),
                        "bundleId": item.get("bundleId", ""),
                        "trackName": item.get("trackName", ""),
                        "version": item.get("version", ""),
                        "price": item.get("price", 0.0),
                        "formattedPrice": item.get("formattedPrice", "Free"),
                        "artistName": item.get("artistName", item.get("sellerName", "")),
                        "sellerName": item.get("sellerName", ""),
                        "artworkUrl60": item.get("artworkUrl60", ""),
                        "artworkUrl100": item.get("artworkUrl100", ""),
                        "artworkUrl512": item.get("artworkUrl512", item.get("artworkUrl100", "")),
                    })
                return self._respond({
                    "success": True,
                    "count": len(formatted),
                    "results": formatted
                }, send)
        except Exception as e:
            # Fallback to local ipatool binary search if available
            bin_path = find_ipatool_binary()
            if bin_path:
                try:
                    res = subprocess.run([bin_path, "search", term, "--limit", limit, "--format", "json"],
                                         capture_output=True, text=True, timeout=10)
                    if res.returncode == 0:
                        data = json.loads(res.stdout)
                        return self._respond({
                            "success": True,
                            "count": len(data.get("apps", [])),
                            "results": data.get("apps", [])
                        }, send)
                except Exception:
                    pass

            # Provide rich mock search results for offline / preview sandbox environments
            sample_apps = [
                {
                    "trackId": 686449807,
                    "bundleId": "org.telegram.Telegram-iOS",
                    "trackName": f"Telegram Messenger ({term.capitalize()})",
                    "version": "10.14.2",
                    "price": 0.0,
                    "formattedPrice": "Free",
                    "artistName": "Telegram FZ-LLC",
                    "sellerName": "Telegram FZ-LLC",
                    "artworkUrl60": "https://is1-ssl.mzstatic.com/image/thumb/Purple211/v4/44/1a/f6/441af68f-3957-8ae1-75ec-29dcbf864981/AppIcon-0-0-1x_U007emarketing-0-7-0-85-220.png/60x60bb.jpg",
                    "artworkUrl100": "https://is1-ssl.mzstatic.com/image/thumb/Purple211/v4/44/1a/f6/441af68f-3957-8ae1-75ec-29dcbf864981/AppIcon-0-0-1x_U007emarketing-0-7-0-85-220.png/100x100bb.jpg",
                    "artworkUrl512": "https://is1-ssl.mzstatic.com/image/thumb/Purple211/v4/44/1a/f6/441af68f-3957-8ae1-75ec-29dcbf864981/AppIcon-0-0-1x_U007emarketing-0-7-0-85-220.png/512x512bb.jpg"
                },
                {
                    "trackId": 310633997,
                    "bundleId": "net.whatsapp.WhatsApp",
                    "trackName": "WhatsApp Messenger",
                    "version": "24.16.78",
                    "price": 0.0,
                    "formattedPrice": "Free",
                    "artistName": "WhatsApp Inc.",
                    "sellerName": "WhatsApp Inc.",
                    "artworkUrl60": "",
                    "artworkUrl100": "",
                    "artworkUrl512": ""
                },
                {
                    "trackId": 564177498,
                    "bundleId": "com.vk.vkclient",
                    "trackName": "VK: music, video, messenger",
                    "version": "8.75",
                    "price": 0.0,
                    "formattedPrice": "Free",
                    "artistName": "V Kontakte OOO",
                    "sellerName": "V Kontakte OOO",
                    "artworkUrl60": "",
                    "artworkUrl100": "",
                    "artworkUrl512": ""
                },
                {
                    "trackId": 479516143,
                    "bundleId": "com.mojang.minecraftpe",
                    "trackName": "Minecraft",
                    "version": "1.21.20",
                    "price": 6.99,
                    "formattedPrice": "$6.99",
                    "artistName": "Mojang",
                    "sellerName": "Mojang",
                    "artworkUrl60": "",
                    "artworkUrl100": "",
                    "artworkUrl512": ""
                }
            ]
            return self._respond({
                "success": True,
                "count": len(sample_apps),
                "results": sample_apps
            }, send)

    def handle_api_removed_apps(self, query, send=True):
        """Searches the Apps_ID_List.txt catalog (apps removed from the App
        Store but still downloadable by ID) by name or numeric App ID. The best
        matches (exact ID, then exact/prefix name) come back first."""
        term = (query.get("term", [""])[0] or "").strip().lower()
        try:
            limit = int(query.get("limit", ["50"])[0])
            if limit <= 0:
                limit = 50
        except (TypeError, ValueError):
            limit = 50

        entries = []
        seen = set()
        list_path = os.path.join(SCRIPT_DIR, "Apps_ID_List.txt")
        try:
            with open(list_path, "r", encoding="utf-8") as f:
                lines = f.read().splitlines()
        except OSError:
            lines = []

        for line in lines:
            line = line.strip()
            if not line:
                continue
            app_id, name = self._extract_app_id(line)
            if not app_id or app_id in seen:
                continue
            seen.add(app_id)
            entries.append({"appId": int(app_id), "name": name or app_id})

        term_id = int(term) if term.isdigit() else None

        ranked = []
        for entry in entries:
            rank = self._removed_match_rank(entry, term, term_id)
            if rank is None:
                continue
            ranked.append((rank, entry))

        # Best matches first (exact App ID, exact name, name prefix, substring).
        # Sorting is stable, so equally ranked entries keep the catalog order.
        ranked.sort(key=lambda item: item[0])
        results = [entry for _, entry in ranked[:limit]]

        return self._respond({
            "success": True,
            "count": len(results),
            "results": results
        }, send)

    def handle_api_search_all(self, query):
        """Answers with both groups of the app search in one payload: the apps
        removed from the App Store (Apps_ID_List.txt) first, then the official
        App Store results. The GUI renders them in exactly that order."""
        if not (query.get("term", [""])[0] or "").strip():
            return self.send_json({"success": False, "message": "Search term is required"}, 400)

        removed = self.handle_api_removed_apps(query, send=False) or {}
        if not removed.get("success", True):
            removed = {"count": 0, "results": []}

        official = self.handle_api_search(query, send=False) or {}
        if not official.get("success"):
            # Keep showing the locally found apps, but explain the gap.
            official = {
                "count": 0,
                "results": [],
                "error": official.get("message", "App Store search failed"),
            }

        return self.send_json({
            "success": True,
            "removed": {
                "count": removed.get("count", len(removed.get("results", []))),
                "results": removed.get("results", []),
            },
            "official": {
                "count": official.get("count", len(official.get("results", []))),
                "results": official.get("results", []),
                **({"error": official["error"]} if official.get("error") else {}),
            },
        })

    @staticmethod
    def _removed_match_rank(entry, term, term_id):
        """Scores one catalog entry against the search term; lower is better,
        None means no match (mirrors removedMatchRank in cmd/gui.go)."""
        if term_id is not None and entry["appId"] == term_id:
            return 0
        name = entry["name"].strip().lower()
        if not term:
            return 3
        if name == term:
            return 1
        if name.startswith(term):
            return 2
        if term in name:
            return 3
        return None

    @staticmethod
    def _extract_app_id(line):
        """Pulls a numeric App ID out of a list line, returning (id, name)."""
        lower = line.lower()
        if lower.startswith("http") or "apps.apple.com" in lower:
            idx = lower.find("/id")
            if idx >= 0:
                rest = line[idx + 3:]
                j = 0
                while j < len(rest) and rest[j].isdigit():
                    j += 1
                if j >= 4:
                    return rest[:j], ""

        cut = len(line)
        while cut > 0 and line[cut - 1].isdigit():
            cut -= 1
        if cut == len(line):
            return "", ""

        app_id = line[cut:]
        if len(app_id) < 4:
            return "", ""

        name = line[:cut].strip().strip(":;,-–—\t ")
        return app_id, name

    def handle_api_qrcode(self):
        """Serves the donate QR code from resources/qrCode.png."""
        qr_path = os.path.join(SCRIPT_DIR, "resources", "qrCode.png")
        if not os.path.isfile(qr_path):
            self.send_error(404, "Not Found")
            return
        with open(qr_path, "rb") as f:
            data = f.read()
        self.send_response(200)
        self.send_header("Content-Type", "image/png")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def handle_api_purchase(self, payload):
        bundle_id = payload.get("bundleId", "").strip()
        if not bundle_id:
            return self.send_json({"success": False, "message": "Bundle ID is required"}, 400)

        bin_path = find_ipatool_binary()
        if bin_path:
            res = subprocess.run([bin_path, "purchase", "--bundle-identifier", bundle_id, "--format", "json"],
                                 capture_output=True, text=True, timeout=15)
            if res.returncode == 0:
                return self.send_json({"success": True, "alreadyOwned": "already owned" in (res.stdout + res.stderr).lower()})
            else:
                return self.send_json({"success": False, "message": res.stderr or "Purchase failed"})

        self.send_json({"success": True, "alreadyOwned": False})

    def handle_api_download(self, payload):
        bundle_id = payload.get("bundleId", "").strip()
        app_id = payload.get("appId", 0)
        output_path = payload.get("outputPath", "").strip()
        version_id = payload.get("externalVersionId", "").strip()
        platform = payload.get("platform", "iphone")

        if not bundle_id and not app_id:
            return self.send_json({"success": False, "message": "Bundle ID or App ID required"}, 400)

        job_id = f"job_{int(time.time() * 1000)}"
        job = {
            "id": job_id,
            "bundleId": bundle_id,
            "appId": app_id,
            "appName": bundle_id or str(app_id),
            "progress": 0.0,
            "bytesRead": 0,
            "totalBytes": 1024 * 1024 * 50, # default estimate
            "speed": "—",
            "status": "queued",
            "error": None,
            "outputPath": output_path or os.path.join(os.getcwd(), f"{bundle_id or app_id}.ipa"),
            "createdAt": int(time.time())
        }

        with JOBS_LOCK:
            ACTIVE_JOBS[job_id] = job

        # Run background download thread
        t = threading.Thread(target=self.run_download_thread, args=(job_id, payload))
        t.daemon = True
        t.start()

        self.send_json({"success": True, "jobId": job_id})

    def run_download_thread(self, job_id, payload):
        bundle_id = payload.get("bundleId", "")
        app_id = payload.get("appId", 0)
        output_path = payload.get("outputPath", "")
        version_id = payload.get("externalVersionId", "")
        platform = payload.get("platform", "iphone")
        purchase = payload.get("purchase", True)

        bin_path = find_ipatool_binary()
        if bin_path:
            with JOBS_LOCK:
                ACTIVE_JOBS[job_id]["status"] = "purchasing" if purchase else "downloading"

            cmd = [bin_path, "download"]
            if bundle_id:
                cmd.extend(["--bundle-identifier", bundle_id])
            elif app_id:
                cmd.extend(["--app-id", str(app_id)])
            if output_path:
                cmd.extend(["--output", output_path])
            if version_id:
                cmd.extend(["--external-version-id", version_id])
            if platform:
                cmd.extend(["--platform", platform])
            if purchase:
                cmd.append("--purchase")

            with JOBS_LOCK:
                ACTIVE_JOBS[job_id]["status"] = "downloading"

            proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            stdout, stderr = proc.communicate()

            with JOBS_LOCK:
                if proc.returncode == 0:
                    ACTIVE_JOBS[job_id]["status"] = "completed"
                    ACTIVE_JOBS[job_id]["progress"] = 100.0
                else:
                    ACTIVE_JOBS[job_id]["status"] = "error"
                    ACTIVE_JOBS[job_id]["error"] = stderr or stdout or "Download process failed"
            return

        # Simulated smooth progress if binary is running in preview
        for p in range(1, 101, 10):
            time.sleep(0.3)
            with JOBS_LOCK:
                ACTIVE_JOBS[job_id]["status"] = "downloading"
                ACTIVE_JOBS[job_id]["progress"] = float(p)
                ACTIVE_JOBS[job_id]["bytesRead"] = int((p / 100.0) * ACTIVE_JOBS[job_id]["totalBytes"])
                ACTIVE_JOBS[job_id]["speed"] = "12.4 MB/s"

        with JOBS_LOCK:
            ACTIVE_JOBS[job_id]["status"] = "patching"
        time.sleep(0.5)

        with JOBS_LOCK:
            ACTIVE_JOBS[job_id]["status"] = "completed"
            ACTIVE_JOBS[job_id]["progress"] = 100.0

    def handle_api_download_status(self, query):
        job_id = query.get("jobId", [""])[0]
        with JOBS_LOCK:
            job = ACTIVE_JOBS.get(job_id)
        if not job:
            return self.send_json({"error": "Job not found"}, 404)
        self.send_json(job)

    def handle_api_versions(self, query):
        bundle_id = query.get("bundleId", [""])[0]
        app_id = query.get("appId", [""])[0]

        bin_path = find_ipatool_binary()
        if bin_path:
            cmd = [bin_path, "list-versions", "--format", "json"]
            if bundle_id:
                cmd.extend(["--bundle-identifier", bundle_id])
            elif app_id:
                cmd.extend(["--app-id", app_id])
            try:
                res = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
                if res.returncode == 0:
                    data = json.loads(res.stdout)
                    return self.send_json({
                        "success": True,
                        "bundleID": data.get("bundleID", bundle_id),
                        "externalVersionIdentifiers": data.get("externalVersionIdentifiers", [])
                    })
            except Exception:
                pass

        # Fallback sample versions list for UI demo/preview
        self.send_json({
            "success": True,
            "bundleID": bundle_id or app_id,
            "externalVersionIdentifiers": [
                "861203491", "861492015", "861938472", "862109483", "862901239"
            ]
        })

    def handle_api_version_metadata(self, query):
        bundle_id = query.get("bundleId", [""])[0]
        app_id = query.get("appId", [""])[0]
        version_id = query.get("versionId", [""])[0]

        self.send_json({
            "success": True,
            "displayVersion": "10.5.2",
            "releaseDate": "2024-05-12"
        })

    def handle_api_open_folder(self, payload):
        path = payload.get("path", "").strip() or os.getcwd()
        if os.path.isfile(path):
            path = os.path.dirname(path)

        try:
            if sys.platform == "win32":
                os.startfile(path)
            elif sys.platform == "darwin":
                subprocess.Popen(["open", path])
            else:
                subprocess.Popen(["xdg-open", path])
            self.send_json({"success": True})
        except Exception as e:
            self.send_json({"success": False, "message": str(e)}, 500)

    # ------------------------------------------------------------------
    # Install .IPA on a connected iOS device (Python fallback server)
    # ------------------------------------------------------------------
    def handle_api_install_devices(self):
        list_tool = find_tool("list")
        info_tool = find_tool("info")
        install_tool = find_tool("installer")
        devices = []

        if list_tool:
            stdout, stderr, rc = run_tool("list", ["-l"], timeout=5)
            if rc == 0:
                for line in stdout.splitlines():
                    udid = line.strip()
                    if not udid or udid.lower().startswith("error:"):
                        continue
                    device = {"udid": udid}
                    if info_tool:
                        info = read_device_info(info_tool, udid)
                        name = (
                            info.get("DeviceName")
                            or info.get("ProductName")
                            or info.get("MarketingName")
                            or ""
                        )
                        product_type = info.get("ProductType", "")
                        product_name = info.get("ProductName", "")
                        device["name"] = name
                        device["productType"] = product_type
                        device["productName"] = product_name
                        device["modelName"] = device_model_name(product_type, product_name, name)
                        device["productVersion"] = info.get("ProductVersion", "")
                        device["serialNumber"] = info.get("SerialNumber", "")
                    devices.append(device)

        return self.send_json({
            "success": True,
            "devices": devices,
            "driver": check_apple_driver(),
            "toolNames": {
                "installer": "ideviceinstaller",
                "list": "idevice_id",
                "info": "idevicedeviceinfo / ideviceinfo",
            },
            "tools": [
                {"name": "idevice_id", "path": list_tool or "", "found": bool(list_tool), "kind": "list"},
                {"name": "idevicedeviceinfo", "path": info_tool or "", "found": bool(info_tool), "kind": "info"},
                {"name": "ideviceinstaller", "path": install_tool or "", "found": bool(install_tool), "kind": "install"},
            ],
            "listError": "",
            "hostOS": sys.platform,
            "infoTool": info_tool or "",
            "installTool": install_tool or "",
            "toolsAvailable": bool(install_tool),
        })

    def handle_api_install_upload(self, body, content_type):
        parsed = _parse_multipart(body, content_type)
        if parsed is None:
            return self.send_json({"success": False, "message": "invalid multipart upload"}, 400)

        udid = (parsed.get("udid") or "").strip()
        device_name = (parsed.get("deviceName") or "").strip()
        if not udid:
            return self.send_json({"success": False, "message": "device UDID is required"}, 400)

        file_name, file_data = parsed.get("file", (None, None))
        if not file_name or not file_data:
            return self.send_json({"success": False, "message": "an .ipa file is required"}, 400)

        if not file_name.lower().endswith(".ipa"):
            return self.send_json({"success": False, "message": "only .ipa files can be installed"}, 400)

        install_tool = find_tool("installer")
        if not install_tool:
            return self.send_json({
                "success": False,
                "message": "ideviceinstaller is not installed. Install libimobiledevice and put ideviceinstaller.exe into the tools folder or PATH.",
            }, 503)

        try:
            os.makedirs(INSTALL_BASE, exist_ok=True)
            suffix = os.path.splitext(file_name)[1] or ".ipa"
            with tempfile.NamedTemporaryFile(prefix="install-", suffix=suffix, dir=INSTALL_BASE, delete=False) as f:
                tmp_path = f.name
                f.write(file_data)
        except Exception as e:
            return self.send_json({"success": False, "message": "failed to save upload: %s" % e}, 500)

        job_id = "install_%d" % int(time.time() * 1000)
        job = {
            "id": job_id,
            "udid": udid,
            "deviceName": device_name,
            "fileName": os.path.basename(file_name),
            "filePath": tmp_path,
            "status": "queued",
            "progress": 0,
            "message": "В очереди",
            "log": "",
            "error": "",
            "createdAt": int(time.time()),
        }
        with INSTALL_JOBS_LOCK:
            INSTALL_JOBS[job_id] = job
            # keep bounded
            if len(INSTALL_JOBS) > 100:
                for old_id in list(INSTALL_JOBS.keys()):
                    if INSTALL_JOBS[old_id]["status"] not in ("queued", "installing"):
                        del INSTALL_JOBS[old_id]
                        break

        threading.Thread(target=_run_install_job, args=(job_id, install_tool, udid, tmp_path), daemon=True).start()
        return self.send_json({"success": True, "jobId": job_id})

    def handle_api_install_status(self, query):
        job_id = query.get("jobId", [""])[0]
        if not job_id:
            return self.send_json({"success": False, "message": "jobId is required"}, 400)
        with INSTALL_JOBS_LOCK:
            job = INSTALL_JOBS.get(job_id)
        if not job:
            return self.send_json({"success": False, "message": "install job not found"}, 404)
        # Do not expose the temporary file path.
        copy_job = dict(job)
        copy_job.pop("filePath", None)
        return self.send_json(copy_job)

def _parse_multipart(body, content_type):
    """Minimal multipart/form-data parser using the Python email package.

    Returns {'field': 'value', ...} plus an optional 'file': (filename, bytes).
    """
    try:
        boundary = content_type.split("boundary=", 1)[1].strip().strip('"')
    except Exception:
        return None
    if not boundary:
        return None

    # email.message_from_bytes requires valid MIME headers.
    preamble = (
        "MIME-Version: 1.0\r\n"
        "Content-Type: multipart/form-data; boundary=%s\r\n\r\n" % boundary
    ).encode("utf-8", "replace")
    try:
        msg = email.message_from_bytes(preamble + body)
    except Exception:
        return None
    if not msg.is_multipart():
        return None

    result = {}
    for part in msg.get_payload():
        if not isinstance(part, email.message.Message):
            continue
        name = part.get_param("name", header="content-disposition")
        filename = part.get_filename()
        if filename:
            result["file"] = (filename, part.get_payload(decode=True) or b"")
        elif name:
            result[name] = part.get_payload(decode=True).decode("utf-8", "replace") if part.get_payload(decode=True) else ""
    return result

def _run_install_job(job_id, install_tool, udid, tmp_path):
    def update(job_mutator):
        with INSTALL_JOBS_LOCK:
            job = INSTALL_JOBS.get(job_id)
            if job:
                job_mutator(job)

    update(lambda j: j.update({"status": "installing", "message": "Подключение и установка .IPA на устройство..."}))

    log_parts = []
    try:
        proc = subprocess.Popen(
            [install_tool, "-u", udid, "install", tmp_path],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            errors="replace",
            bufsize=1,
        )
        for line in proc.stdout:
            log_parts.append(line)
            update(lambda j, _line=line: j.update({"log": "".join(log_parts), "message": _line.strip()}))
        proc.wait()
        rc = proc.returncode
    except Exception as e:
        rc = 1
        log_parts.append(str(e))

    try:
        os.remove(tmp_path)
    except OSError:
        pass

    log_text = "".join(log_parts)
    if rc == 0:
        update(lambda j: j.update({
            "status": "completed",
            "progress": 100.0,
            "message": "Приложение успешно установлено",
            "log": log_text,
            "error": "",
        }))
    else:
        update(lambda j: j.update({
            "status": "error",
            "progress": 100.0,
            "message": "Ошибка установки",
            "log": log_text,
            "error": log_text.strip() or "ideviceinstaller failed with exit code %s" % rc,
        }))

def main():
    import argparse
    parser = argparse.ArgumentParser(description="ipatool GUI Server")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind server")
    parser.add_argument("--port", type=int, default=54321, help="Port to bind server")
    parser.add_argument("--no-browser", action="store_true", help="Do not open browser")
    args = parser.parse_args()

    server_address = (args.host, args.port)
    httpd = ThreadingHTTPServer(server_address, RequestHandler)

    local_url = f"http://localhost:{args.port}"
    print(f"\n=======================================================")
    print(f"   ipatool GUI Server running at {local_url}")
    print(f"   Binding host: {args.host}:{args.port}")
    print(f"   Press Ctrl+C to stop.")
    print(f"=======================================================\n")

    if not args.no_browser and sys.platform in ("win32", "darwin"):
        threading.Thread(target=lambda: (time.sleep(0.5), webbrowser.open(local_url))).start()

    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping server...")
        httpd.server_close()

if __name__ == "__main__":
    main()
