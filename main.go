package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
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
	downloadFolderAbsPath string,
	fileExtensions []string,
	responseChan <-chan playwright.Response,
	counterChan chan<- int,
) {
	counter := 0

	for response := range responseChan {
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
	}
	fmt.Printf("%s✓ Response channel closed, worker exiting%s\n", Cyan, Reset)
}

// printCounterWorker listens to counter updates and displays the current count on a single line.
// It uses carriage return (\r) to overwrite the same line on each update.
func printCounterWorker(counterChan <-chan int) {
	for counter := range counterChan {
		fmt.Printf("\r%s%sTotal files downloaded: %d%s    ", Cyan, Bold, counter, Reset)
	}
}

// parseCookie parses a document.cookie string into a slice of Playwright cookies for the given page URL.
func parseCookie(documentCookie string, pageUrl string) []playwright.OptionalCookie {
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
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run holds the actual program logic. Returning an error here (instead of
// calling log.Fatal) means every defer registered below always runs, so the
// browser process is never leaked when something fails partway through.
func run() error {
	// ========================================
	// 0. Parse CLI Input
	// ========================================

	// Mandatory
	url := flag.String("url", "", "URL to open in browser")
	fileExtensionsStr := flag.String("file-extensions", "", "Comma-separated list of file extensions to download (e.g., .vtt,.srt,.mp4)")

	// Optional
	configFilePath := flag.String("config", "", "Path to browser config file (.json, .yaml, or .yml) (Optional)")
	cookieFilePath := flag.String("cookie-file", "", "Path to cookie file (Optional)")
	browserFlag := flag.String(
		"browser",
		"chromium",
		"Browser to use (Optional). Possible values: firefox, chromium, webkit. Defaults to chromium.",
	)
	confirmRecord := flag.Bool("confirm-record", false, "Confimr before recording the download. Useful if you want to record only after performing certain action in the browser(Optional)")

	// Optional, to be entered later
	withCookie := flag.Bool("with-cookie", false, "With this flag, the program will ask you to enter a cookie (Optional)")
	downloadFolderPathFlag := flag.String(
		"download-folder",
		"",
		"Folder to download all the files to (Optional). "+
			"If this param is missing the program with ask you to enter the path manually. "+
			"The path can be both absolute or relative path.",
	)

	flag.Parse()

	// Validate required flags
	if *url == "" {
		fmt.Printf("%s✗ Error: --url flag is required%s\n", Red, Reset)
		return fmt.Errorf("usage: network-file-downloader --url <URL> --file-extensions <extensions> [--config <path>] [--cookie <path>]")
	}

	if *fileExtensionsStr == "" {
		fmt.Printf("%s✗ Error: --file-extensions flag is required%s\n", Red, Reset)
		return fmt.Errorf("usage: network-file-downloader --url <URL> --file-extensions <extensions> [--config <path>] [--cookie <path>]")
	}

	// --config and --browser are mutually exclusive: when a config file is used,
	// the browser must be selected via the config's "browser" field.
	browserFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "browser" {
			browserFlagSet = true
		}
	})
	if *configFilePath != "" && browserFlagSet {
		fmt.Printf("%s✗ Error: --browser cannot be used together with --config; set \"browser\" in the config file instead%s\n", Red, Reset)
		return fmt.Errorf("usage: network-file-downloader --url <URL> --file-extensions <extensions> [--config <path> | --browser firefox|chromium|webkit]")
	}

	// Ask user for input on optional parameter with empty value
	downloadFolderPath := *downloadFolderPathFlag
	if downloadFolderPath == "" {
		fmt.Printf("%s%sInput folder path to download to (e.g., . for current directory):%s ", Bold, Yellow, Reset)
		fmt.Scan(&downloadFolderPath)
	}
	if err := os.MkdirAll(downloadFolderPath, 0755); err != nil {
		fmt.Printf("%s✗ Error: could not create download folder %q: %v%s\n", Red, downloadFolderPath, err, Reset)
		return fmt.Errorf("usage: network-file-downloader --url <URL> --file-extensions <extensions> [--download-folder <path>]")
	}
	downloadFolderAbsPath, err := filepath.Abs(downloadFolderPath)
	if err != nil {
		fmt.Printf("%s✗ Error: could not resolve download folder path: %v%s\n", Red, err, Reset)
		return err
	}

	// Load browser config
	cfg := defaultBrowserConfig()
	if *configFilePath != "" {
		loaded, err := loadConfig(*configFilePath)
		if err != nil {
			fmt.Printf("%s✗ Error: could not load config: %v%s\n", Red, err, Reset)
			return err
		}
		applyDefaults(&loaded, cfg)
		cfg = loaded
		fmt.Printf("%s✓ Loaded config from: %s%s\n", Green, *configFilePath, Reset)
	}

	if browserFlagSet {
		cfg.Browser = *browserFlag
	}

	switch cfg.Browser {
	case "firefox", "chromium", "webkit":
	default:
		fmt.Printf("%s✗ Error: invalid browser value %q (must be firefox, chromium, or webkit)%s\n", Red, cfg.Browser, Reset)
		return fmt.Errorf("usage: network-file-downloader --url <URL> --file-extensions <extensions> [--browser firefox|chromium|webkit]")
	}

	// Parse file extensions, dropping empty entries (e.g. from a trailing/double
	// comma) since an empty extension would match every response's URL.
	var fileExtensions []string
	for _, ext := range strings.Split(*fileExtensionsStr, ",") {
		ext = strings.TrimSpace(ext)
		if ext != "" {
			fileExtensions = append(fileExtensions, ext)
		}
	}
	if len(fileExtensions) == 0 {
		fmt.Printf("%s✗ Error: --file-extensions must contain at least one non-empty extension%s\n", Red, Reset)
		return fmt.Errorf("usage: network-file-downloader --url <URL> --file-extensions <extensions>")
	}

	// ========================================
	// 1. Init channels
	// ========================================
	counterChan := make(chan int, 10)
	defer close(counterChan)

	browserResponseChan := make(chan playwright.Response, 100)
	defer close(browserResponseChan)

	// ========================================
	// 2. Initialize Browser
	// ========================================
	pw, err := playwright.Run()
	if err != nil {
		fmt.Printf("%s✗ Error: could not start Playwright: %v%s\n", Red, err, Reset)
		return err
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
		fmt.Printf("%s✗ Error: could not launch browser: %v%s\n", Red, err, Reset)
		return err
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
		fmt.Printf("%s✗ Error: could not create context: %v%s\n", Red, err, Reset)
		return err
	}
	defer context.Close()

	if *cookieFilePath != "" {
		if *withCookie {
			fmt.Printf("%s⚠ Both --cookie-file and --with-cookie were provided; using the cookie from --cookie-file%s\n", Yellow, Reset)
		}
		cookieContentBytes, err := os.ReadFile(*cookieFilePath)
		if err != nil {
			fmt.Printf("%s✗ Error: could not read cookie file: %v%s\n", Red, err, Reset)
			return err
		}
		cookieContent := string(cookieContentBytes)
		optionalCookies := parseCookie(cookieContent, *url)
		if err := context.AddCookies(optionalCookies); err != nil {
			fmt.Printf("%s✗ Error: could not add cookie: %v%s\n", Red, err, Reset)
			return err
		}
	}

	if *cookieFilePath == "" && *withCookie {
		fmt.Printf("%s%sPlease input your cookie:%s ", Bold, Yellow, Reset)
		reader := bufio.NewReader(os.Stdin)
		cookieContent, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("%s✗ Error: could not read cookie input: %v%s\n", Red, err, Reset)
			return err
		}
		cookieContent = strings.TrimSpace(cookieContent)
		optionalCookies := parseCookie(cookieContent, *url)
		if err := context.AddCookies(optionalCookies); err != nil {
			fmt.Printf("%s✗ Error: could not add cookie: %v%s\n", Red, err, Reset)
			return err
		}
	}

	page, err := context.NewPage()
	if err != nil {
		fmt.Printf("%s✗ Error: could not create page: %v%s\n", Red, err, Reset)
		return err
	}
	defer page.Close()

	if _, err = page.Goto(*url); err != nil {
		fmt.Printf("%s✗ Error: could not visit this url: %v%s\n", Red, err, Reset)
		return err
	}

	fmt.Printf("%s%s✓ Browser opened successfully!%s Press Ctrl+C to exit...\n", Bold, Green, Reset)
	fmt.Printf("%sMonitoring file extensions: %s%s%s\n", Cyan, Yellow, strings.Join(fileExtensions, ", "), Reset)

	// ========================================
	// 3. Start Workers and Handlers
	// ========================================

	// Start response worker
	go responseWorker(
		downloadFolderAbsPath,
		fileExtensions,
		browserResponseChan,
		counterChan,
	)

	go printCounterWorker(counterChan)

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
	if *confirmRecord {
		fmt.Printf("%s%sPress Enter to confirm start recording...%s\n", Bold, Yellow, Reset)
		fmt.Scanln()
	}

	fmt.Printf("%s%s✓ Recording started!%s Saving files to: %s%s%s\n", Bold, Green, Reset, Cyan, downloadFolderAbsPath, Reset)

	fmt.Printf("\n%s%sPress Ctrl+C to stop recording...%s\n", Bold, Cyan, Reset)

	isRecording.Store(true)
	counterChan <- 0

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Printf("\n%s✓ Recording stopped%s\n", Green, Reset)

	return nil
}
