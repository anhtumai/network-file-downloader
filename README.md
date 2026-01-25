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
network-file-downloader --url <URL> [--file-extension <extensions>]
```

### Options

- `--url`: URL to open in browser (required)
- `--file-extension`: Comma-separated list of file extensions to download (default: `.vtt`)

### Examples

Download `.vtt` subtitle files:
```bash
network-file-downloader -url https://example.com/video
```

Download multiple file types:
```bash
network-file-downloader -url https://example.com/video -file-extension .vtt,.srt,.mp4
```

### Interactive Mode

Once the browser opens:

1. The tool will prompt: **Start Recording (y/n):**
2. Enter `y` or `yes` to begin
3. Specify a folder path to save downloaded files
4. The tool monitors network traffic and saves matching files
5. Press **Enter** to stop recording
6. Repeat or press **Ctrl+C** to exit

## How It Works

The tool uses Playwright to launch a real Firefox browser and intercepts network responses. When a response URL matches one of the specified file extensions, it automatically saves the file to your chosen directory.

This is particularly useful for:
- Downloading subtitle files (.vtt, .srt)
- Capturing media segments
- Downloading resources that are loaded dynamically
- Accessing files behind authentication or complex JavaScript

## Requirements

- Go 1.21 or higher (for building from source)
- Playwright Firefox browser (automatically installed with `go run` or `go build`)

## License

MIT License - see LICENSE file for details
