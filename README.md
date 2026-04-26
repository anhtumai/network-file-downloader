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
go build -o network-file-downloader main.go
```

## Usage

```bash
network-file-downloader --url <URL> --file-extension <extensions> [--config <path>] [--cookie <path>]
```

### Options

- `--url`: URL to open in browser (required)
- `--file-extension`: Comma-separated list of file extensions to download (required, e.g. `.vtt,.srt,.mp4`)
- `--config`: Path to a browser config file (`.json`, `.yaml`, or `.yml`)
- `--cookie`: Path to a cookie file (document.cookie format)

### Examples

Download `.vtt` subtitle files:
```bash
network-file-downloader --url https://example.com/video --file-extension .vtt
```

Download multiple file types:
```bash
network-file-downloader --url https://example.com/video --file-extension .vtt,.srt,.mp4
```

Use a custom browser config:
```bash
network-file-downloader --url https://example.com/video --config ./config.json
```

Use cookies for authenticated sessions:
```bash
network-file-downloader --url https://example.com/video --cookie ./cookie.txt
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

Then pass it with `--cookie ./cookie.txt`. Cookies are injected into the browser context before the page loads.

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

Once the browser opens:

1. The tool will prompt: **Start Recording (y/n):**
2. Enter `y` or `yes` to begin
3. Specify a folder path to save downloaded files
4. The tool monitors network traffic and saves matching files
5. Press **Enter** to stop recording
6. Repeat or press **Ctrl+C** to exit

## How It Works

The tool uses Playwright to launch a real browser and intercepts network responses. When a response URL matches one of the specified file extensions, it automatically saves the file to your chosen directory.

This is particularly useful for:
- Downloading subtitle files (.vtt, .srt)
- Capturing media segments
- Downloading resources that are loaded dynamically
- Accessing files behind authentication or complex JavaScript

## Requirements

- Go 1.21 or higher (for building from source)
- Playwright browser (automatically installed with `go run` or `go build`)

## License

MIT License - see LICENSE file for details
