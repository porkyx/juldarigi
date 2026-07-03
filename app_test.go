package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestAppScrapeDCGalleryTrimsDefaultsAndEmits(t *testing.T) {
	t.Parallel()

	fake := &fakeScrapeService{
		fn: func(ctx context.Context, request ScrapeRequest, emit EventEmitter) (*ScrapeResult, error) {
			if request.URL != "https://gall.dcinside.com/mini/vsoop" {
				t.Fatalf("URL = %q, want trimmed URL", request.URL)
			}
			if request.Pages != 1 {
				t.Fatalf("Pages = %d, want default 1", request.Pages)
			}
			emit("progress", ProgressInfo{CurrentPage: 1, Message: "ok"})
			return &ScrapeResult{Success: true, GalleryID: "vsoop"}, nil
		},
	}

	var emitted []string
	app := &App{
		ctx:     context.Background(),
		scraper: fake,
		emit: func(ctx context.Context, eventName string, optionalData ...interface{}) {
			emitted = append(emitted, eventName)
		},
	}

	result, err := app.ScrapeDCGallery(ScrapeRequest{URL: "  https://gall.dcinside.com/mini/vsoop  "})
	if err != nil {
		t.Fatalf("ScrapeDCGallery returned error: %v", err)
	}
	if result.GalleryID != "vsoop" {
		t.Fatalf("GalleryID = %q, want vsoop", result.GalleryID)
	}
	if len(emitted) != 1 || emitted[0] != "scrape:progress" {
		t.Fatalf("emitted = %+v, want scrape:progress", emitted)
	}
}

func TestAppScrapeDCGalleryValidation(t *testing.T) {
	t.Parallel()

	app := NewApp()

	tests := []struct {
		name    string
		request ScrapeRequest
		want    string
	}{
		{name: "empty URL", request: ScrapeRequest{}, want: "URL is required"},
		{
			name:    "date range reversed",
			request: ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop", StartDate: "2026-07-03", EndDate: "2026-07-01"},
			want:    "start date must be earlier than or equal to end date",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := app.ScrapeDCGallery(test.request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAppRejectsMissingScraper(t *testing.T) {
	t.Parallel()

	app := &App{}
	_, err := app.ScrapeDCGallery(ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop"})
	if err == nil || !strings.Contains(err.Error(), "scraper is not configured") {
		t.Fatalf("error = %v, want scraper configuration error", err)
	}
}

func TestAppRejectsConcurrentScrapeAndCancelStopsActiveRun(t *testing.T) {
	blocking := &blockingScrapeService{
		started: make(chan struct{}),
		done:    make(chan error, 1),
	}
	app := &App{
		ctx:     context.Background(),
		scraper: blocking,
		emit:    func(ctx context.Context, eventName string, optionalData ...interface{}) {},
	}

	firstResult := make(chan error, 1)
	go func() {
		_, err := app.ScrapeDCGallery(ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop", Pages: 1})
		firstResult <- err
	}()

	<-blocking.started

	_, err := app.ScrapeDCGallery(ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop", Pages: 1})
	if err == nil || !strings.Contains(err.Error(), "scraping is already in progress") {
		t.Fatalf("concurrent error = %v, want already in progress", err)
	}

	if cancelled := app.CancelScrape(); !cancelled {
		t.Fatal("CancelScrape returned false, want true")
	}
	if err := <-firstResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("first scrape error = %v, want context.Canceled", err)
	}
	if err := <-blocking.done; !errors.Is(err, context.Canceled) {
		t.Fatalf("blocking scraper done error = %v, want context.Canceled", err)
	}
	if cancelled := app.CancelScrape(); cancelled {
		t.Fatal("second CancelScrape returned true, want false")
	}
}

func TestAppShutdownCancelsActiveScrape(t *testing.T) {
	blocking := &blockingScrapeService{
		started: make(chan struct{}),
		done:    make(chan error, 1),
	}
	app := &App{
		ctx:     context.Background(),
		scraper: blocking,
		emit:    func(ctx context.Context, eventName string, optionalData ...interface{}) {},
	}

	firstResult := make(chan error, 1)
	go func() {
		_, err := app.ScrapeDCGallery(ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop", Pages: 1})
		firstResult <- err
	}()
	<-blocking.started

	app.shutdown(context.Background())

	if err := <-firstResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("first scrape error = %v, want context.Canceled", err)
	}
}

func TestAppEmitScrapeEventNoopsWithoutContextOrEmitter(t *testing.T) {
	t.Parallel()

	(&App{}).emitScrapeEvent("progress", ProgressInfo{})
	(&App{ctx: context.Background()}).emitScrapeEvent("progress", ProgressInfo{})
}

func TestAppSaveCaptureImageWritesPNGWithSaveDialogPath(t *testing.T) {
	t.Parallel()

	pngData := minimalPNGData()
	var dialogOptions runtime.SaveDialogOptions
	var writtenPath string
	var writtenData []byte
	var writtenPerm os.FileMode
	app := &App{
		ctx: context.Background(),
		saveFileDialog: func(ctx context.Context, options runtime.SaveDialogOptions) (string, error) {
			dialogOptions = options
			return filepath.Join(t.TempDir(), "capture"), nil
		},
		writeFile: func(name string, data []byte, perm os.FileMode) error {
			writtenPath = name
			writtenData = append([]byte(nil), data...)
			writtenPerm = perm
			return nil
		},
	}

	savedPath, err := app.SaveCaptureImage(SaveCaptureRequest{
		DataURL:         capturePNGDataURL(pngData),
		DefaultFilename: `bad:name?.png`,
	})
	if err != nil {
		t.Fatalf("SaveCaptureImage returned error: %v", err)
	}
	if savedPath != writtenPath {
		t.Fatalf("saved path = %q, written path = %q", savedPath, writtenPath)
	}
	if filepath.Ext(savedPath) != ".png" {
		t.Fatalf("saved path = %q, want .png extension", savedPath)
	}
	if string(writtenData) != string(pngData) {
		t.Fatalf("written data = %v, want %v", writtenData, pngData)
	}
	if writtenPerm != 0o644 {
		t.Fatalf("written perm = %v, want 0644", writtenPerm)
	}
	if dialogOptions.Title == "" || len(dialogOptions.Filters) != 1 || dialogOptions.Filters[0].Pattern != "*.png" {
		t.Fatalf("dialog options = %+v, want PNG save dialog", dialogOptions)
	}
	if strings.ContainsAny(dialogOptions.DefaultFilename, `\/:*?"<>|`) {
		t.Fatalf("default filename contains reserved chars: %q", dialogOptions.DefaultFilename)
	}
}

func TestAppSaveCaptureImageCancelDoesNotWrite(t *testing.T) {
	t.Parallel()

	wrote := false
	app := &App{
		ctx: context.Background(),
		saveFileDialog: func(ctx context.Context, options runtime.SaveDialogOptions) (string, error) {
			return "", nil
		},
		writeFile: func(name string, data []byte, perm os.FileMode) error {
			wrote = true
			return nil
		},
	}

	path, err := app.SaveCaptureImage(SaveCaptureRequest{DataURL: capturePNGDataURL(minimalPNGData())})
	if err != nil {
		t.Fatalf("SaveCaptureImage returned error: %v", err)
	}
	if path != "" {
		t.Fatalf("cancel path = %q, want empty", path)
	}
	if wrote {
		t.Fatal("writeFile was called after save dialog cancellation")
	}
}

func TestAppSaveCaptureImageValidationAndFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		app     *App
		request SaveCaptureRequest
		want    string
	}{
		{
			name:    "missing context",
			app:     &App{saveFileDialog: func(ctx context.Context, options runtime.SaveDialogOptions) (string, error) { return "x", nil }, writeFile: os.WriteFile},
			request: SaveCaptureRequest{DataURL: capturePNGDataURL(minimalPNGData())},
			want:    "application context is not ready",
		},
		{
			name:    "missing dialog",
			app:     &App{ctx: context.Background(), writeFile: os.WriteFile},
			request: SaveCaptureRequest{DataURL: capturePNGDataURL(minimalPNGData())},
			want:    "save dialog is not configured",
		},
		{
			name: "missing writer",
			app: &App{
				ctx:            context.Background(),
				saveFileDialog: func(ctx context.Context, options runtime.SaveDialogOptions) (string, error) { return "x", nil },
			},
			request: SaveCaptureRequest{DataURL: capturePNGDataURL(minimalPNGData())},
			want:    "file writer is not configured",
		},
		{
			name:    "not data URL",
			app:     captureTestApp(t, nil, nil),
			request: SaveCaptureRequest{DataURL: "plain"},
			want:    "capture image must be a PNG data URL",
		},
		{
			name:    "empty data",
			app:     captureTestApp(t, nil, nil),
			request: SaveCaptureRequest{DataURL: capturePNGDataURLPrefix},
			want:    "capture image data is empty",
		},
		{
			name:    "invalid base64",
			app:     captureTestApp(t, nil, nil),
			request: SaveCaptureRequest{DataURL: capturePNGDataURLPrefix + "%%%"},
			want:    "capture image data is invalid",
		},
		{
			name:    "not png",
			app:     captureTestApp(t, nil, nil),
			request: SaveCaptureRequest{DataURL: capturePNGDataURL([]byte("not png"))},
			want:    "capture image data is not PNG",
		},
		{
			name: "dialog failure",
			app: captureTestApp(t, func(ctx context.Context, options runtime.SaveDialogOptions) (string, error) {
				return "", errors.New("dialog failed")
			}, nil),
			request: SaveCaptureRequest{DataURL: capturePNGDataURL(minimalPNGData())},
			want:    "dialog failed",
		},
		{
			name: "write failure",
			app: captureTestApp(t, nil, func(name string, data []byte, perm os.FileMode) error {
				return errors.New("disk full")
			}),
			request: SaveCaptureRequest{DataURL: capturePNGDataURL(minimalPNGData())},
			want:    "disk full",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := test.app.SaveCaptureImage(test.request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestDecodeCapturePNGDataURLRejectsOversizedImages(t *testing.T) {
	t.Parallel()

	t.Run("encoded data exceeds limit", func(t *testing.T) {
		t.Parallel()

		data := append(minimalPNGData(), make([]byte, maxCapturePNGBytes)...)
		encoded := strings.TrimPrefix(capturePNGDataURL(data), capturePNGDataURLPrefix)
		if len(encoded) <= maxCapturePNGBase64Len {
			t.Fatalf("encoded length = %d, want greater than %d", len(encoded), maxCapturePNGBase64Len)
		}
		_, err := decodeCapturePNGDataURL(capturePNGDataURL(data))
		if err == nil || !strings.Contains(err.Error(), "capture image is too large") {
			t.Fatalf("error = %v, want oversized image error", err)
		}
	})

	t.Run("decoded data exceeds limit", func(t *testing.T) {
		t.Parallel()

		data := append(minimalPNGData(), make([]byte, maxCapturePNGBytes+1-len(minimalPNGData()))...)
		encoded := strings.TrimPrefix(capturePNGDataURL(data), capturePNGDataURLPrefix)
		if len(encoded) > maxCapturePNGBase64Len {
			t.Fatalf("encoded length = %d, want at most %d", len(encoded), maxCapturePNGBase64Len)
		}
		_, err := decodeCapturePNGDataURL(capturePNGDataURL(data))
		if err == nil || !strings.Contains(err.Error(), "capture image is too large") {
			t.Fatalf("error = %v, want oversized image error", err)
		}
	})
}

func TestAppStartupStoresContext(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), testContextKey{}, "ready")
	app := &App{}
	app.startup(ctx)
	if app.ctx.Value(testContextKey{}) != "ready" {
		t.Fatal("startup did not store context")
	}
}

func TestAppScrapeUsesBackgroundWhenStartupDidNotRun(t *testing.T) {
	t.Parallel()

	fake := &fakeScrapeService{
		fn: func(ctx context.Context, request ScrapeRequest, emit EventEmitter) (*ScrapeResult, error) {
			if ctx == nil {
				t.Fatal("context is nil, want background context")
			}
			return &ScrapeResult{Success: true}, nil
		},
	}
	app := &App{scraper: fake}
	if _, err := app.ScrapeDCGallery(ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop", Pages: 1}); err != nil {
		t.Fatalf("ScrapeDCGallery returned error: %v", err)
	}
}

func TestNewAppHasDefaults(t *testing.T) {
	t.Parallel()

	app := NewApp()
	if app.scraper == nil {
		t.Fatal("NewApp scraper is nil")
	}
	if app.emit == nil {
		t.Fatal("NewApp emitter is nil")
	}
}

type testContextKey struct{}

type fakeScrapeService struct {
	fn func(ctx context.Context, request ScrapeRequest, emit EventEmitter) (*ScrapeResult, error)
}

func (f *fakeScrapeService) Scrape(ctx context.Context, request ScrapeRequest, emit EventEmitter) (*ScrapeResult, error) {
	return f.fn(ctx, request, emit)
}

type blockingScrapeService struct {
	once    sync.Once
	started chan struct{}
	done    chan error
}

func (b *blockingScrapeService) Scrape(ctx context.Context, request ScrapeRequest, emit EventEmitter) (*ScrapeResult, error) {
	b.once.Do(func() {
		close(b.started)
	})
	<-ctx.Done()
	b.done <- ctx.Err()
	return nil, ctx.Err()
}

func minimalPNGData() []byte {
	return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
}

func capturePNGDataURL(data []byte) string {
	return capturePNGDataURLPrefix + base64.StdEncoding.EncodeToString(data)
}

func captureTestApp(t *testing.T, dialog saveFileDialogFunc, writer writeFileFunc) *App {
	t.Helper()
	if dialog == nil {
		dialog = func(ctx context.Context, options runtime.SaveDialogOptions) (string, error) {
			return filepath.Join(t.TempDir(), "capture.png"), nil
		}
	}
	if writer == nil {
		writer = func(name string, data []byte, perm os.FileMode) error {
			return nil
		}
	}
	return &App{
		ctx:            context.Background(),
		saveFileDialog: dialog,
		writeFile:      writer,
	}
}
