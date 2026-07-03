package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	capturePNGDataURLPrefix = "data:image/png;base64,"
	maxCapturePNGBytes      = 25 * 1024 * 1024
	maxCapturePNGBase64Len  = ((maxCapturePNGBytes + 2) / 3) * 4
)

type scrapeService interface {
	Scrape(ctx context.Context, request ScrapeRequest, emit EventEmitter) (*ScrapeResult, error)
}

type runtimeEventEmitter func(ctx context.Context, eventName string, optionalData ...interface{})
type saveFileDialogFunc func(ctx context.Context, dialogOptions runtime.SaveDialogOptions) (string, error)
type writeFileFunc func(name string, data []byte, perm os.FileMode) error

type SaveCaptureRequest struct {
	DataURL         string `json:"dataUrl"`
	DefaultFilename string `json:"defaultFilename"`
}

type App struct {
	ctx            context.Context
	scraper        scrapeService
	emit           runtimeEventEmitter
	saveFileDialog saveFileDialogFunc
	writeFile      writeFileFunc

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewApp() *App {
	return &App{
		scraper:        NewScraper(nil),
		emit:           runtime.EventsEmit,
		saveFileDialog: runtime.SaveFileDialog,
		writeFile:      os.WriteFile,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	a.CancelScrape()
}

func (a *App) ScrapeDCGallery(request ScrapeRequest) (*ScrapeResult, error) {
	request.URL = strings.TrimSpace(request.URL)
	request.StartDate = strings.TrimSpace(request.StartDate)
	request.EndDate = strings.TrimSpace(request.EndDate)

	if request.URL == "" {
		return nil, errors.New("URL is required")
	}
	if request.StartDate != "" && request.EndDate != "" && request.StartDate > request.EndDate {
		return nil, errors.New("start date must be earlier than or equal to end date")
	}
	if request.StartDate == "" && request.EndDate == "" && request.Pages < 1 {
		request.Pages = 1
	}
	if a.scraper == nil {
		return nil, errors.New("scraper is not configured")
	}

	baseContext := a.ctx
	if baseContext == nil {
		baseContext = context.Background()
	}

	scrapeContext, cancel := context.WithCancel(baseContext)
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		cancel()
		return nil, errors.New("scraping is already in progress")
	}
	a.cancel = cancel
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.cancel = nil
		a.mu.Unlock()
	}()

	return a.scraper.Scrape(scrapeContext, request, a.emitScrapeEvent)
}

func (a *App) CancelScrape() bool {
	a.mu.Lock()
	cancel := a.cancel
	a.cancel = nil
	a.mu.Unlock()

	if cancel == nil {
		return false
	}

	cancel()
	a.emitScrapeEvent("warning", MessagePayload{Message: "Scraping cancelled"})
	return true
}

func (a *App) SaveCaptureImage(request SaveCaptureRequest) (string, error) {
	if a.ctx == nil {
		return "", errors.New("application context is not ready")
	}
	if a.saveFileDialog == nil {
		return "", errors.New("save dialog is not configured")
	}
	if a.writeFile == nil {
		return "", errors.New("file writer is not configured")
	}

	data, err := decodeCapturePNGDataURL(request.DataURL)
	if err != nil {
		return "", err
	}

	targetPath, err := a.saveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "캡처 이미지 저장",
		DefaultFilename: defaultCaptureFilename(request.DefaultFilename),
		Filters: []runtime.FileFilter{
			{DisplayName: "PNG Image (*.png)", Pattern: "*.png"},
		},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(targetPath) == "" {
		return "", nil
	}

	targetPath = ensurePNGExtension(targetPath)
	if err := a.writeFile(targetPath, data, 0o644); err != nil {
		return "", fmt.Errorf("save capture image: %w", err)
	}
	return targetPath, nil
}

func (a *App) emitScrapeEvent(eventName string, payload interface{}) {
	if a.ctx == nil || a.emit == nil {
		return
	}
	a.emit(a.ctx, "scrape:"+eventName, payload)
}

func decodeCapturePNGDataURL(dataURL string) ([]byte, error) {
	trimmed := strings.TrimSpace(dataURL)
	if !strings.HasPrefix(trimmed, capturePNGDataURLPrefix) {
		return nil, errors.New("capture image must be a PNG data URL")
	}

	encoded := strings.TrimSpace(strings.TrimPrefix(trimmed, capturePNGDataURLPrefix))
	if encoded == "" {
		return nil, errors.New("capture image data is empty")
	}
	if len(encoded) > maxCapturePNGBase64Len {
		return nil, fmt.Errorf("capture image is too large: encoded data exceeds %d characters", maxCapturePNGBase64Len)
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("capture image data is invalid")
	}
	if len(data) > maxCapturePNGBytes {
		return nil, fmt.Errorf("capture image is too large: %d bytes", len(data))
	}
	if !hasPNGSignature(data) {
		return nil, errors.New("capture image data is not PNG")
	}
	return data, nil
}

func hasPNGSignature(data []byte) bool {
	pngSignature := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	return bytes.HasPrefix(data, pngSignature)
}

func defaultCaptureFilename(value string) string {
	safe := strings.TrimSpace(value)
	if safe == "" {
		safe = "juldarigi_capture"
	}
	safe = strings.Map(func(char rune) rune {
		switch char {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		default:
			return char
		}
	}, safe)
	return ensurePNGExtension(safe)
}

func ensurePNGExtension(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return path
	}
	return path + ".png"
}
