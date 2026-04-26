package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/playwright-community/playwright-go"
	"gopkg.in/yaml.v3"
)

// ViewportSize holds browser viewport dimensions.
type ViewportSize struct {
	Width  int `json:"width"  yaml:"width"`
	Height int `json:"height" yaml:"height"`
}

// BrowserConfig holds all configurable browser options.
// Fields that affect anti-bot detection are marked accordingly.
type BrowserConfig struct {
	Browser           string            `json:"browser"             yaml:"browser"`             // firefox | chromium | webkit
	BrowserChannel    string            `json:"browser_channel"     yaml:"browser_channel"`     // e.g. "chrome", "msedge" (chromium only)
	UserAgent         string            `json:"user_agent"          yaml:"user_agent"`          // high impact
	Locale            string            `json:"locale"              yaml:"locale"`              // high impact: navigator.language
	TimezoneId        string            `json:"timezone_id"         yaml:"timezone_id"`         // high impact
	Viewport          *ViewportSize     `json:"viewport"            yaml:"viewport"`            // high impact
	DeviceScaleFactor float64           `json:"device_scale_factor" yaml:"device_scale_factor"` // high impact
	HasTouch          bool              `json:"has_touch"           yaml:"has_touch"`           // medium impact
	ColorScheme       string            `json:"color_scheme"        yaml:"color_scheme"`        // medium: light|dark|no-preference
	Permissions       []string          `json:"permissions"         yaml:"permissions"`
	ExtraHttpHeaders  map[string]string `json:"extra_http_headers"  yaml:"extra_http_headers"`
}

// defaultBrowserConfig returns the defaults matching the original hardcoded behaviour.
func defaultBrowserConfig() BrowserConfig {
	return BrowserConfig{
		Browser:     "firefox",
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Permissions: []string{"geolocation", "notifications"},
	}
}

// loadConfig reads a JSON or YAML config file and returns a BrowserConfig.
// Zero-value fields are filled with defaults afterwards by the caller.
func loadConfig(filePath string) (BrowserConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return BrowserConfig{}, fmt.Errorf("reading config: %w", err)
	}

	var cfg BrowserConfig
	switch {
	case strings.HasSuffix(filePath, ".json"):
		err = json.Unmarshal(data, &cfg)
	case strings.HasSuffix(filePath, ".yaml"), strings.HasSuffix(filePath, ".yml"):
		err = yaml.Unmarshal(data, &cfg)
	default:
		return BrowserConfig{}, fmt.Errorf("unsupported config format (use .json, .yaml, or .yml)")
	}
	if err != nil {
		return BrowserConfig{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// applyDefaults fills zero-value fields in cfg with values from defaults.
func applyDefaults(cfg *BrowserConfig, defaults BrowserConfig) {
	if cfg.Browser == "" {
		cfg.Browser = defaults.Browser
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaults.UserAgent
	}
	if cfg.Permissions == nil {
		cfg.Permissions = defaults.Permissions
	}
}

// ANSI color codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Bold   = "\033[1m"
)

// responseWorker listens to the responses channel and saves files matching the specified extensions to disk.
// It runs continuously until the channel is closed.
func responseWorker(
	responses <-chan playwright.Response,
	downloadFolderAbsPathChan <-chan string,
	counterChan chan<- int,
	fileExtensions []string,
) {
	downloadFolderAbsPath := "."

	counter := 0

	for {
		select {
		case response, ok := <-responses:
			if !ok {
				fmt.Printf("%s✓ Response channel closed, worker exiting%s\n", Cyan, Reset)
				return
			}
			responseUrl := response.URL()

			// Check if URL ends with any of the specified extensions
			matchesExtension := false
			fileNameInResponseUrl := strings.SplitN(responseUrl, "?", 2)[0]
			for _, ext := range fileExtensions {
				if strings.HasSuffix(fileNameInResponseUrl, ext) {
					matchesExtension = true
					break
				}
			}

			if matchesExtension {
				body, err := response.Text()
				if err != nil {
					fmt.Printf("%s✗ Error reading body: %v%s\n", Red, err, Reset)
					continue
				}
				fileName := path.Base(fileNameInResponseUrl)
				filePath := filepath.Join(downloadFolderAbsPath, fileName)
				if err := os.WriteFile(filePath, []byte(body), 0644); err != nil {
					fmt.Printf("%s✗ Error writing file %s: %v%s\n", Red, fileName, err, Reset)
				} else {
					counter++
					counterChan <- counter
				}
			}

		case downloadFolderAbsPathValue, ok := <-downloadFolderAbsPathChan:
			if !ok {
				fmt.Printf("%s✓ Download folder channel closed, worker exiting%s\n", Cyan, Reset)
				return
			}
			downloadFolderAbsPath = downloadFolderAbsPathValue
			// Reset counter since download folder changed meaning a new session
			counter = 0
		}
	}
}

// printCounterWorker listens to counter updates and displays the current count on a single line.
// It uses carriage return (\r) to overwrite the same line on each update.
func printCounterWorker(counterChan <-chan int) {
	for counter := range counterChan {
		fmt.Printf("\r%s%sTotal files downloaded: %d%s    ", Cyan, Bold, counter, Reset)
	}
}

// validateAndPrepareFolder converts a relative or absolute path to an absolute path
// and validates that it exists and is a directory.
func validateAndPrepareFolder(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot access folder: %v", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path exists but is not a directory")
	}

	return absPath, nil
}

// ParseCookie parses a document.cookie string into a slice of Playwright cookies for the given page URL.
func ParseCookie(documentCookie string, pageUrl string) []playwright.OptionalCookie {
	result := []playwright.OptionalCookie{}
	cookieParts := strings.Split(documentCookie, ";")
	for _, cookiePart := range cookieParts {
		cookieKeyValue := strings.SplitN(strings.TrimSpace(cookiePart), "=", 2)
		if len(cookieKeyValue) < 2 {
			continue
		}
		name := strings.TrimSpace(cookieKeyValue[0])
		value := strings.TrimSpace(cookieKeyValue[1])
		if name == "" {
			continue
		}
		optionalCookie := playwright.OptionalCookie{
			Name:  name,
			Value: value,
			URL:   &pageUrl,
		}
		result = append(result, optionalCookie)
	}

	return result
}

func main() {
	// ========================================
	// 1. Parse CLI Input
	// ========================================
	url := flag.String("url", "", "URL to open in browser")
	fileExtensionsStr := flag.String("file-extension", "", "Comma-separated list of file extensions to download (e.g., .vtt,.srt,.mp4)")
	configPath := flag.String("config", "", "Path to browser config file (.json, .yaml, or .yml)")
	cookiePath := flag.String("cookie", "", "Path to cookie file")

	flag.Parse()

	// Validate required flags
	if *url == "" {
		fmt.Printf("%s✗ Error: --url flag is required%s\n", Red, Reset)
		log.Fatal("Usage: network-file-downloader --url <URL> --file-extension <extensions> [--config <path>] [--cookie <path>]")
	}

	if *fileExtensionsStr == "" {
		fmt.Printf("%s✗ Error: --file-extension flag is required%s\n", Red, Reset)
		log.Fatal("Usage: network-file-downloader --url <URL> --file-extension <extensions> [--config <path>] [--cookie <path>]")
	}

	// Load browser config
	cfg := defaultBrowserConfig()
	if *configPath != "" {
		loaded, err := loadConfig(*configPath)
		if err != nil {
			log.Fatalf("could not load config: %v", err)
		}
		applyDefaults(&loaded, cfg)
		cfg = loaded
		fmt.Printf("%s✓ Loaded config from: %s%s\n", Green, *configPath, Reset)
	}

	// Parse file extensions
	fileExtensions := strings.Split(*fileExtensionsStr, ",")
	for i := range fileExtensions {
		fileExtensions[i] = strings.TrimSpace(fileExtensions[i])
	}

	// ========================================
	// 2. Initialize Browser
	// ========================================
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start Playwright: %v", err)
	}
	defer pw.Stop()

	var browserType playwright.BrowserType
	switch cfg.Browser {
	case "chromium":
		browserType = pw.Chromium
	case "webkit":
		browserType = pw.WebKit
	default:
		browserType = pw.Firefox
	}

	launchOpts := playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
	}
	if cfg.BrowserChannel != "" {
		launchOpts.Channel = playwright.String(cfg.BrowserChannel)
	}

	browser, err := browserType.Launch(launchOpts)
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	contextOpts := playwright.BrowserNewContextOptions{
		Permissions: cfg.Permissions,
		UserAgent:   playwright.String(cfg.UserAgent),
	}
	if cfg.Locale != "" {
		contextOpts.Locale = playwright.String(cfg.Locale)
	}
	if cfg.TimezoneId != "" {
		contextOpts.TimezoneId = playwright.String(cfg.TimezoneId)
	}
	if cfg.Viewport != nil {
		contextOpts.Viewport = &playwright.Size{Width: cfg.Viewport.Width, Height: cfg.Viewport.Height}
	}
	if cfg.DeviceScaleFactor != 0 {
		contextOpts.DeviceScaleFactor = playwright.Float(cfg.DeviceScaleFactor)
	}
	if cfg.HasTouch {
		contextOpts.HasTouch = playwright.Bool(true)
	}
	if cfg.ColorScheme != "" {
		cs := playwright.ColorScheme(cfg.ColorScheme)
		contextOpts.ColorScheme = &cs
	}
	if len(cfg.ExtraHttpHeaders) > 0 {
		contextOpts.ExtraHttpHeaders = cfg.ExtraHttpHeaders
	}

	context, err := browser.NewContext(contextOpts)
	if err != nil {
		log.Fatalf("could not create context: %v", err)
	}
	defer context.Close()

	if *cookiePath != "" {
		cookieContentBytes, err := os.ReadFile(*cookiePath)
		if err != nil {
			log.Fatalf("could not read cookie file: %v", err)
		}
		cookieContent := string(cookieContentBytes)
		optionalCookies := ParseCookie(cookieContent, *url)

		err = context.AddCookies(optionalCookies)
		if err != nil {
			log.Fatalf("could not add cookie: %v", err)
		}

	}

	page, err := context.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	defer page.Close()

	if _, err = page.Goto(*url); err != nil {
		log.Fatalf("could not visit this url: %v", err)
	}

	fmt.Printf("%s%s✓ Browser opened successfully!%s Press Ctrl+C to exit...\n", Bold, Green, Reset)
	fmt.Printf("%sMonitoring file extensions: %s%s%s\n", Cyan, Yellow, strings.Join(fileExtensions, ", "), Reset)

	// ========================================
	// 3. Start Workers and Handlers
	// ========================================
	browserResponseChan := make(chan playwright.Response, 100)
	defer close(browserResponseChan)

	downloadFolderAbsPathChan := make(chan string, 1)
	defer close(downloadFolderAbsPathChan)

	counterChan := make(chan int, 10)
	defer close(counterChan)

	// Start response worker
	go responseWorker(browserResponseChan, downloadFolderAbsPathChan, counterChan, fileExtensions)

	go printCounterWorker(counterChan)

	// Send initial download folder path
	// downloadFolderAbsPathChan <- downloadAbsolutePath

	// Register response handler
	var isRecording atomic.Bool
	page.OnResponse(func(response playwright.Response) {
		if isRecording.Load() {
			browserResponseChan <- response
		}
	})

	// ========================================
	// 4. User Interaction
	// ========================================
	for {

		fmt.Printf("%s%sStart Recording (y/n):%s ", Bold, Yellow, Reset)
		var startRecordingInput string
		fmt.Scan(&startRecordingInput)

		if startRecordingInput != "y" && startRecordingInput != "yes" {
			fmt.Printf("%s⚠ Recording cancelled%s\n", Yellow, Reset)
			return
		}

		var downloadAbsolutePath string
		for {
			fmt.Printf("%s%sInput folder path to download to (e.g., . for current directory):%s ", Bold, Yellow, Reset)
			var downloadFolderPathInput string
			fmt.Scan(&downloadFolderPathInput)
			downloadFolderPathInput = strings.TrimSpace(downloadFolderPathInput)

			_downloadAbsolutePath, err := validateAndPrepareFolder(downloadFolderPathInput)
			if err != nil {
				fmt.Printf("%s✗ Cannot open folder: %v%s\n", Red, err, Reset)
			} else {
				downloadAbsolutePath = _downloadAbsolutePath
				break
			}
		}

		fmt.Printf("%s%s✓ Recording started!%s Saving files to: %s%s%s\n", Bold, Green, Reset, Cyan, downloadAbsolutePath, Reset)

		fmt.Printf("\n%s%sPress Enter to stop recording...%s\n", Bold, Cyan, Reset)

		isRecording.Store(true)
		downloadFolderAbsPathChan <- downloadAbsolutePath
		counterChan <- 0

		fmt.Scanln()

		fmt.Printf("\n%s✓ Recording stopped%s\n", Green, Reset)
		isRecording.Store(false)

	}
}
