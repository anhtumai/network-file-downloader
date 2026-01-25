package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/playwright-community/playwright-go"
)

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
	fileExtensions []string,
) {
	downloadFolderAbsPath := "."

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
			for _, ext := range fileExtensions {
				if strings.HasSuffix(responseUrl, ext) {
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
				fileName := path.Base(responseUrl)
				filePath := filepath.Join(downloadFolderAbsPath, fileName)
				if err := os.WriteFile(filePath, []byte(body), 0644); err != nil {
					fmt.Printf("%s✗ Error writing file %s: %v%s\n", Red, fileName, err, Reset)
				}
			}

		case downloadFolderAbsPathValue, ok := <-downloadFolderAbsPathChan:
			if !ok {
				fmt.Printf("%s✓ Download folder channel closed, worker exiting%s\n", Cyan, Reset)
				return
			}
			downloadFolderAbsPath = downloadFolderAbsPathValue
		}
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
	if os.IsNotExist(err) {
		return "", fmt.Errorf("cannot access folder: %v", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path exists but is not a directory")
	}

	return absPath, nil
}

func main() {
	// ========================================
	// 1. Parse CLI Input
	// ========================================
	url := flag.String("url", "", "URL to open in browser")
	fileExtensionsStr := flag.String("file-extension", ".vtt", "Comma-separated list of file extensions to download (e.g., .vtt,.srt,.mp4)")
	flag.Parse()

	// Validate required flags
	if *url == "" {
		fmt.Printf("%s✗ Error: --url flag is required%s\n", Red, Reset)
		log.Fatal("Usage: network-file-downloader --url <URL> [--file-extension <extensions>]")
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

	browser, err := pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		Permissions: []string{"geolocation", "notifications"},
		UserAgent:   playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		Viewport:    nil,
	})
	defer page.Close()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

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

	// Start response worker
	go responseWorker(browserResponseChan, downloadFolderAbsPathChan, fileExtensions)

	// Send initial download folder path
	// downloadFolderAbsPathChan <- downloadAbsolutePath

	// Register response handler
	isRecording := false
	page.OnResponse(func(response playwright.Response) {
		if isRecording {
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
		isRecording = true
		downloadFolderAbsPathChan <- downloadAbsolutePath

		fmt.Printf("%s%sPress Enter to stop recording...%s ", Bold, Cyan, Reset)
		fmt.Scanln()

		fmt.Printf("%s✓ Recording stopped%s\n", Green, Reset)
		isRecording = false

	}
}
