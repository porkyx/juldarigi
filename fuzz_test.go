package main

import (
	"strings"
	"testing"
	"time"
)

func FuzzParseGalleryURL(f *testing.F) {
	for _, seed := range []string{
		"",
		"https://gall.dcinside.com/mini/vsoop",
		"gall.dcinside.com/mgallery/test_gallery",
		"https://gall.dcinside.com/board/lists/?id=baseball_new11",
		"https://example.com/not-dc",
		"http://[::1",
		strings.Repeat("a", 1024),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, rawURL string) {
		info, err := ParseGalleryURL(rawURL)
		if err != nil {
			return
		}
		if info.GalleryID == "" {
			t.Fatalf("valid URL produced empty gallery ID: %+v", info)
		}
		if info.GalleryType != "mini" && info.GalleryType != "mgallery" && info.GalleryType != "board" {
			t.Fatalf("valid URL produced invalid gallery type: %+v", info)
		}
		pageURL := BuildPageURL(info, 1)
		if !strings.Contains(pageURL, "gall.dcinside.com") || !strings.Contains(pageURL, "page=1") {
			t.Fatalf("page URL lost required fields: %q", pageURL)
		}
	})
}

func FuzzNormalizeDateWithNow(f *testing.F) {
	for _, seed := range []string{
		"",
		"2026-07-03 12:00",
		"2026.07.03 12:00",
		"26/06/29",
		"12:30",
		"07.03",
		"bad-date",
		"9999-99-99",
		strings.Repeat("1", 256),
	} {
		f.Add(seed)
	}

	now := time.Date(2026, 7, 3, 14, 0, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, input string) {
		normalized := normalizeDateWithNow(input, now)
		if normalized == "" {
			return
		}
		if len(normalized) != len("2006-01-02") {
			t.Fatalf("normalized date length = %d, want 10 for %q", len(normalized), normalized)
		}
		if normalized[4] != '-' || normalized[7] != '-' {
			t.Fatalf("normalized date has invalid separators: %q", normalized)
		}
	})
}
