package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/playwright-community/playwright-go"
)

type Event struct {
	Type     string
	FileName string
}

// responseWorker listens to the responses channel and saves all .vtt files to disk.
// It runs continuously until the channel is closed.
func responseWorker(
	responses <-chan playwright.Response,
	downloadFolderAbsPathChan <-chan string,
) {
	fmt.Println("Response worker started")

	downloadFolderAbsPath := "."

	for {
		select {
		case response := <-responses:
			responseUrl := response.URL()

			if strings.HasSuffix(responseUrl, ".vtt") {
				// Update global state
				body, err := response.Text()
				if err != nil {
					fmt.Printf("Error reading body: %v\n", err)
					return
				}
				fileName := path.Base(responseUrl)
				filePath := fmt.Sprintf("%s/%s", downloadFolderAbsPath, fileName)
				os.WriteFile(filePath, []byte(body), 0644)
			}

		case downloadFolderAbsPathValue := <-downloadFolderAbsPathChan:
			downloadFolderAbsPath = downloadFolderAbsPathValue

		}
	}
}

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
	_ = fmt.Println // suppress "not used" error
	_ = os.Args     // suppress "not used" error

	url := flag.String("url", "", "URL to open")
	// downloadFolder := flag.String("download-folder", "", "Folder to download files to")
	flag.Parse()

	fmt.Println("Url:", *url)
	// fileExtensions := []string{".vtt"}

	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start Playwrght: %v", err)
	}
	defer pw.Stop()
browser, err := pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{ Headless: playwright.Bool(false), }) if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		Permissions: []string{"geolocation", "notifications"},
		UserAgent:   playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		Viewport: &playwright.Size{
			Width:  1920,
			Height: 1080,
		},
		Locale:     playwright.String("fi-FI"), // Finnish locale
		TimezoneId: playwright.String("Europe/Helsinki"),
		Geolocation: &playwright.Geolocation{
			Latitude:  60.1699,
			Longitude: 24.9384,
		},
	})

	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	browserResponseChan := make(chan playwright.Response)
	defer close(browserResponseChan)

	downloadFolderAbsPathChan := make(chan string)

	if _, err = page.Goto(*url); err != nil {
		log.Fatalf("could not visit this url: %v", err)
		os.Exit(1)
	}

	fmt.Println("Browser is running. Press Ctrl+C to exit...")

	isRecording := false
	// Main Thread Command line logic
	fmt.Print("Start Recording (y/n): ")
	var startRecordingInput string
	fmt.Scan(&startRecordingInput)

	if startRecordingInput == "y" || startRecordingInput == "yes" {
		fmt.Println("Recording started!")
	} else {
		fmt.Println("Recording cancelled")
		os.Exit(0)
	}

	fmt.Print("Input folder path to download to: ")
	var downloadFolderPathInput string

	fmt.Scan(&downloadFolderPathInput)
	downloadFolderPathInput = strings.TrimSpace(downloadFolderPathInput)

	downloadAbsolutePath, err := validateAndPrepareFolder(downloadFolderPathInput)
	if err != nil {
		log.Fatalln("Cannot open folder to download: %v", err)
		os.Exit(1)
	}
	isRecording = true
	downloadFolderAbsPathChan <- downloadAbsolutePath

	//
	//

	go responseWorker(browserResponseChan, downloadFolderAbsPathChan)

	page.OnResponse(func(response playwright.Response) {
		if isRecording {
			browserResponseChan <- response
		}
	})

	// Wait for Ctrl + C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}
