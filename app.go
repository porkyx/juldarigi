package main

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type scrapeService interface {
	Scrape(ctx context.Context, request ScrapeRequest, emit EventEmitter) (*ScrapeResult, error)
}

type runtimeEventEmitter func(ctx context.Context, eventName string, optionalData ...interface{})

type App struct {
	ctx     context.Context
	scraper scrapeService
	emit    runtimeEventEmitter

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewApp() *App {
	return &App{
		scraper: NewScraper(nil),
		emit:    runtime.EventsEmit,
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

func (a *App) emitScrapeEvent(eventName string, payload interface{}) {
	if a.ctx == nil || a.emit == nil {
		return
	}
	a.emit(a.ctx, "scrape:"+eventName, payload)
}
