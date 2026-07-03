package main

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveSPVJuneDateRangePostsE2E(t *testing.T) {
	if os.Getenv("JULDARIGI_LIVE_E2E") != "1" {
		t.Skip("set JULDARIGI_LIVE_E2E=1 to run live DCInside e2e")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	result, err := NewScraper(nil).Scrape(ctx, ScrapeRequest{
		URL:            "https://gall.dcinside.com/mini/board/lists/?id=spv",
		StartDate:      "2026-06-01",
		EndDate:        "2026-06-30",
		CollectionMode: collectionModePosts,
	}, nil)
	if err != nil {
		t.Fatalf("live scrape returned error: %v", err)
	}

	t.Logf("pages=%d total_posts=%d unique_users=%d rankings=%d", result.PagesScraped, result.TotalPosts, result.UniqueUsers, len(result.UserStats))
	if result.CollectionMode != collectionModePosts {
		t.Fatalf("collection mode = %q, want %q", result.CollectionMode, collectionModePosts)
	}
	if result.TotalPosts == 0 {
		t.Fatal("live scrape returned zero posts for 2026-06-01..2026-06-30")
	}
	if len(result.UserStats) == 0 {
		t.Fatal("live scrape returned zero user rankings")
	}
	if len(result.UserStats) != result.UniqueUsers {
		t.Fatalf("user ranking length = %d, unique users = %d", len(result.UserStats), result.UniqueUsers)
	}
	if result.UserStats[0].PostCount < 1 || result.UserStats[0].Count != result.UserStats[0].PostCount || result.UserStats[0].CommentCount != 0 {
		t.Fatalf("top ranking has invalid posts-only counters: %+v", result.UserStats[0])
	}
}
