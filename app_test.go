package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
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
