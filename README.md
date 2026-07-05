# network-file-downloader

A command-line tool that uses browser automation to download files from network requests. Monitor and capture files (like subtitles, media files, or any resource) as they're requested by websites.

## Demo

Watch the tool in action: [https://youtu.be/JzI21Cq0mxc](https://youtu.be/JzI21Cq0mxc)

## Features

- Browser-based file downloading using Playwright
- Support for multiple file extensions (`.vtt`, `.srt`, `.mp4`, etc.)
- Interactive recording mode - start/stop capturing on demand
- Colored terminal output for better UX
- Cross-platform support (Linux, macOS, Windows)
- Configurable browser options via JSON/YAML config file
- Cookie injection support for authenticated sessions

## Installation

### Download Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/anhtumai/network-file-downloader/releases).

### Build from Source

```bash
# Clone the repository
git clone https://github.com/anhtumai/network-file-downloader.git
cd network-file-downloader

# Install dependencies
go mod download

# Build the binary
go build -o network-file-downloader .
```

## Usage

```bash
network-file-downloader --url <URL> --file-extensions <extensions> [flags]
```

### Options

- `--url`: URL to open in browser (required)
- `--file-extensions`: Comma-separated list of file extensions to download (required, e.g. `.vtt,.srt,.mp4`)
- `--config`: Path to a browser config file (`.json`, `.yaml`, or `.yml`) (optional)
- `--browser`: Browser to use: `firefox` (default), `chromium`, or `webkit` (optional). Cannot be combined with `--config`; set `browser` in the config file instead
- `--cookie-file`: Path to a cookie file (document.cookie format) (optional)
- `--with-cookie`: Prompt to enter a cookie interactively instead of using `--cookie-file` (optional)
- `--confirm-record`: Wait for Enter to be pressed before starting to record, useful if you want to perform an action in the browser first (optional)
- `--download-folder`: Folder to save downloaded files to, absolute or relative (optional). If omitted, you'll be prompted to enter it interactively

### Examples

Download `.vtt` subtitle files:
```bash
network-file-downloader --url https://example.com/video --file-extensions .vtt
```

Download multiple file types:
```bash
network-file-downloader --url https://example.com/video --file-extensions .vtt,.srt,.mp4
```

Use a custom browser config:
```bash
network-file-downloader --url https://example.com/video --file-extensions .vtt,.srt --config ./config.json
```

Pick a browser without a config file:
```bash
network-file-downloader --url https://example.com/video --file-extensions .vtt,.srt --browser chromium
```

Use cookies for authenticated sessions:
```bash
network-file-downloader --url https://example.com/video --file-extensions .vtt,.srt --cookie-file ./cookie.txt
```

Set the download folder up front and wait for a manual cue before recording:
```bash
network-file-downloader --url https://example.com/video --file-extensions .vtt --download-folder ./downloads --confirm-record
```

### Browser Config File

You can customize browser behavior via a JSON or YAML config file. All fields are optional and fall back to defaults if omitted.

**JSON example (`config.json`):**
```json
{
  "browser": "chromium",
  "browser_channel": "chrome",
  "user_agent": "Mozilla/5.0 ...",
  "locale": "en-US",
  "timezone_id": "America/New_York",
  "viewport": { "width": 1920, "height": 1080 },
  "device_scale_factor": 1.0,
  "has_touch": false,
  "color_scheme": "dark",
  "permissions": ["geolocation", "notifications"],
  "extra_http_headers": {
    "Accept-Language": "en-US,en;q=0.9"
  }
}
```

**YAML example (`config.yaml`):**
```yaml
browser: firefox
user_agent: "Mozilla/5.0 ..."
locale: en-US
timezone_id: Europe/London
viewport:
  width: 1280
  height: 720
```

| Field | Values | Impact |
|---|---|---|
| `browser` | `firefox` (default), `chromium`, `webkit` | — |
| `browser_channel` | `chrome`, `msedge` (chromium only) | — |
| `user_agent` | any string | high |
| `locale` | e.g. `en-US` | high |
| `timezone_id` | e.g. `America/New_York` | high |
| `viewport` | `{ width, height }` | high |
| `device_scale_factor` | float | high |
| `has_touch` | bool | medium |
| `color_scheme` | `light`, `dark`, `no-preference` | medium |
| `permissions` | list of strings | — |
| `extra_http_headers` | key-value map | — |

### Cookie File

To access authenticated content, export your browser cookies as a `document.cookie` string and save it to a file:

```
sessionId=abc123; authToken=xyz789; userId=42
```

Then pass it with `--cookie-file ./cookie.txt`. Cookies are injected into the browser context before the page loads. Alternatively, pass `--with-cookie` to be prompted for the cookie string interactively instead of using a file (note: very long cookies may be truncated by your terminal's paste buffer, so `--cookie-file` is more reliable).

#### How to export cookies from your browser

**Method 1: Browser DevTools (any browser)**

1. Log in to the website in your browser
2. Open DevTools (`F12` or `Ctrl+Shift+I` / `Cmd+Option+I` on Mac)
3. Go to the **Console** tab
4. Run:
   ```js
   copy(document.cookie)
   ```
5. This copies the cookie string to your clipboard — paste it into a file and save it

**Method 2: DevTools Application tab (Chrome / Edge)**

1. Log in to the website
2. Open DevTools → **Application** tab → **Cookies** → select the site
3. You can inspect individual cookies here; to get them all as a string, use the Console method above

**Method 3: DevTools Storage tab (Firefox)**

1. Log in to the website
2. Open DevTools → **Storage** tab → **Cookies** → select the site
3. Use the Console method above to copy all cookies at once

### Interactive Mode

If certain options aren't passed as flags, the tool will prompt for them before opening the browser:

1. If `--download-folder` is omitted, you'll be asked to enter the folder path to save downloaded files to (created automatically if it doesn't exist)
2. If `--with-cookie` is set (and `--cookie-file` isn't), you'll be asked to paste your cookie string
3. The browser then opens and navigates to `--url`
4. If `--confirm-record` is set, the tool waits for **Enter** before it starts recording — useful if you want to perform an action (e.g. log in, start a video) first. Otherwise recording starts immediately
5. The tool monitors network traffic and saves matching files, showing a live count of files downloaded
6. Press **Ctrl+C** to stop recording and exit

## How It Works

The tool uses Playwright to launch a real browser and intercepts network responses. When a response URL matches one of the specified file extensions, it automatically saves the file to your chosen directory.

This is particularly useful for:
- Downloading subtitle files (.vtt, .srt)
- Capturing media segments
- Downloading resources that are loaded dynamically
- Accessing files behind authentication or complex JavaScript

## Requirements

- Go 1.25.6 or higher (for building from source, per `go.mod`)
- Playwright browser (automatically installed with `go run` or `go build`)

## License

MIT License - see LICENSE file for details
