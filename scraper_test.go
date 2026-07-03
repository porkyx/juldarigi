package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func TestParseGalleryURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rawURL      string
		galleryID   string
		galleryType string
	}{
		{
			name:        "mini board list",
			rawURL:      "https://gall.dcinside.com/mini/board/lists/?id=vsoop&page=2",
			galleryID:   "vsoop",
			galleryType: "mini",
		},
		{
			name:        "mini short",
			rawURL:      "https://gall.dcinside.com/mini/vsoop",
			galleryID:   "vsoop",
			galleryType: "mini",
		},
		{
			name:        "mgallery board list",
			rawURL:      "https://gall.dcinside.com/mgallery/board/lists/?id=test_gallery",
			galleryID:   "test_gallery",
			galleryType: "mgallery",
		},
		{
			name:        "board list",
			rawURL:      "https://gall.dcinside.com/board/lists/?id=baseball_new11",
			galleryID:   "baseball_new11",
			galleryType: "board",
		},
		{
			name:        "board short",
			rawURL:      "https://gall.dcinside.com/baseball_new11",
			galleryID:   "baseball_new11",
			galleryType: "board",
		},
		{
			name:        "scheme omitted",
			rawURL:      "gall.dcinside.com/mini/vsoop",
			galleryID:   "vsoop",
			galleryType: "mini",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			info, err := ParseGalleryURL(test.rawURL)
			if err != nil {
				t.Fatalf("ParseGalleryURL returned error: %v", err)
			}
			if info.GalleryID != test.galleryID {
				t.Fatalf("GalleryID = %q, want %q", info.GalleryID, test.galleryID)
			}
			if info.GalleryType != test.galleryType {
				t.Fatalf("GalleryType = %q, want %q", info.GalleryType, test.galleryType)
			}
		})
	}
}

func TestParseGalleryURLRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"",
		"http://[::1",
		"https://example.com/mini/vsoop",
		"https://gall.dcinside.com/mini/board/lists/",
		"https://gall.dcinside.com/mgallery/board/lists/",
		"https://gall.dcinside.com/board/lists/",
		"https://gall.dcinside.com/",
		"https://gall.dcinside.com/mini/board/not-lists/extra",
		"https://gall.dcinside.com/mgallery/board/not-lists/extra",
	} {
		rawURL := rawURL
		t.Run(rawURL, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseGalleryURL(rawURL); err == nil {
				t.Fatalf("ParseGalleryURL(%q) succeeded, want error", rawURL)
			}
		})
	}
}

func TestBuildPageURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		info GalleryInfo
		want string
	}{
		{
			info: GalleryInfo{GalleryID: "hello world", GalleryType: "mini"},
			want: "https://gall.dcinside.com/mini/board/lists/?id=hello+world&page=3",
		},
		{
			info: GalleryInfo{GalleryID: "hello/world", GalleryType: "mgallery"},
			want: "https://gall.dcinside.com/mgallery/board/lists/?id=hello%2Fworld&page=3",
		},
		{
			info: GalleryInfo{GalleryID: "baseball_new11", GalleryType: "board"},
			want: "https://gall.dcinside.com/board/lists/?id=baseball_new11&page=3",
		},
	}

	for _, test := range tests {
		got := BuildPageURL(test.info, 3)
		if got != test.want {
			t.Fatalf("BuildPageURL(%+v) = %q, want %q", test.info, got, test.want)
		}
	}
}

func TestNormalizeDateWithNow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 3, 14, 0, 0, 0, time.Local)
	tests := []struct {
		input string
		want  string
	}{
		{input: "2026-07-02 23:59", want: "2026-07-02"},
		{input: "14:30", want: "2026-07-03"},
		{input: "06.30", want: "2026-06-30"},
		{input: "", want: ""},
		{input: "not-a-date", want: ""},
		{input: "2026/07/02", want: ""},
		{input: "2026-0x-02", want: ""},
		{input: "1x:30", want: ""},
		{input: "0x.30", want: ""},
	}

	for _, test := range tests {
		got := normalizeDateWithNow(test.input, now)
		if got != test.want {
			t.Fatalf("normalizeDateWithNow(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestExtractPostsFromDocumentSkipsNoticeRows(t *testing.T) {
	t.Parallel()

	doc := mustDocument(t, `
		<table><tbody class="listwrap2">
			<tr class="ub-notice"><td class="gall_writer" data-uid="notice"><span class="nickname">공지</span></td></tr>
			<tr><td><em class="icon_img icon_notice"></em></td><td class="gall_writer" data-uid="notice2"><span class="nickname">공지2</span></td></tr>
			<tr><td class="gall_writer" data-uid="u1"><span class="nickname">Alice</span><span class="ip">(1.2)</span></td></tr>
			<tr><td class="gall_writer" data-uid="u2"><span class="nick_comm">Bob</span></td></tr>
			<tr><td class="gall_writer" data-uid="u3"></td></tr>
			<tr><td class="gall_writer"><span class="nickname">No UID</span></td></tr>
		</tbody></table>
	`)

	posts := extractPostsFromDocument(doc)
	if len(posts) != 3 {
		t.Fatalf("len(posts) = %d, want 3", len(posts))
	}
	if posts[0].UID != "u1" || posts[0].Nickname != "Alice" || posts[0].IP != "(1.2)" {
		t.Fatalf("first post = %+v", posts[0])
	}
	if posts[1].UID != "u2" || posts[1].Nickname != "Bob" {
		t.Fatalf("second post = %+v", posts[1])
	}
	if posts[2].UID != "u3" || posts[2].Nickname != "Unknown" {
		t.Fatalf("third post = %+v, want Unknown nickname", posts[2])
	}
}

func TestExtractPostsFromDocumentIncludesListMetrics(t *testing.T) {
	t.Parallel()

	doc := mustDocument(t, galleryHTML(
		metricPostRow("123", "u1", "Alice", "1.2", "First title", "/mini/board/view/?id=spv&no=123&page=1", "1,234", "56", "[12]")+
			metricPostRow("124", "u2", "Bob", "", "No reply title", "https://gall.dcinside.com/mini/board/view/?id=spv&no=124&page=1", "0", "0", ""),
	))

	posts := extractPostsFromDocument(doc)
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want 2", len(posts))
	}

	first := posts[0]
	if first.Number != "123" || first.Title != "First title" {
		t.Fatalf("first post identity = %+v, want number and title", first)
	}
	if first.URL != "https://gall.dcinside.com/mini/board/view/?id=spv&no=123&page=1" {
		t.Fatalf("first URL = %q, want absolute DCInside URL", first.URL)
	}
	if first.Nickname != "Alice" || first.IP != "1.2" {
		t.Fatalf("writer fallback fields = nickname %q ip %q, want Alice and 1.2", first.Nickname, first.IP)
	}
	if !first.HasViewCount || first.ViewCount != 1234 {
		t.Fatalf("view metric = %d/%v, want 1234/true", first.ViewCount, first.HasViewCount)
	}
	if !first.HasRecommendCount || first.RecommendCount != 56 {
		t.Fatalf("recommend metric = %d/%v, want 56/true", first.RecommendCount, first.HasRecommendCount)
	}
	if !first.HasCommentCount || first.CommentCount != 12 {
		t.Fatalf("comment metric = %d/%v, want 12/true", first.CommentCount, first.HasCommentCount)
	}

	second := posts[1]
	if !second.HasCommentCount || second.CommentCount != 0 {
		t.Fatalf("missing reply box comment metric = %d/%v, want 0/true", second.CommentCount, second.HasCommentCount)
	}
	if !second.HasViewCount || second.ViewCount != 0 {
		t.Fatalf("zero view metric = %d/%v, want 0/true", second.ViewCount, second.HasViewCount)
	}
}

func TestParseMetricText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
		ok    bool
	}{
		{name: "empty", input: "", ok: false},
		{name: "dash", input: "-", ok: false},
		{name: "zero", input: "0", want: 0, ok: true},
		{name: "comma", input: "1,234", want: 1234, ok: true},
		{name: "reply brackets", input: "[9]", want: 9, ok: true},
		{name: "no digits", input: "abc", ok: false},
		{name: "overflow", input: strings.Repeat("9", 64), ok: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseMetricText(test.input)
			if got != test.want || ok != test.ok {
				t.Fatalf("parseMetricText(%q) = %d/%v, want %d/%v", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestEnrichPostFromRowHandlesMissingOptionalFields(t *testing.T) {
	t.Parallel()

	post := Post{UID: "u1"}
	enrichPostFromRow(nil, &post)
	if post.Number != "" || post.HasViewCount || post.HasRecommendCount || post.HasCommentCount {
		t.Fatalf("nil row changed post metrics: %+v", post)
	}

	row := mustDocument(t, `
		<table><tbody class="listwrap2">
			<tr>
				<td class="gall_num">10</td>
				<td class="gall_tit ub-word"> Plain title </td>
			</tr>
		</tbody></table>
	`).Find("tr").First()
	enrichPostFromRow(row, nil)

	post = Post{UID: "u2"}
	enrichPostFromRow(row, &post)
	if post.Number != "10" || post.Title != "Plain title" {
		t.Fatalf("post identity = %+v, want number 10 and plain title", post)
	}
	if !post.HasCommentCount || post.CommentCount != 0 {
		t.Fatalf("comment metric = %d/%v, want zero comment count", post.CommentCount, post.HasCommentCount)
	}
	if post.HasViewCount || post.HasRecommendCount {
		t.Fatalf("missing count cells produced metrics: %+v", post)
	}
}

func TestExtractCommentCountBoundaries(t *testing.T) {
	t.Parallel()

	emptySelection := mustDocument(t, `<table></table>`).Find("td.gall_tit").First()
	if count, ok := extractCommentCount(emptySelection); count != 0 || ok {
		t.Fatalf("empty title comment count = %d/%v, want 0/false", count, ok)
	}

	invalidReply := mustDocument(t, `<td class="gall_tit"><span class="reply_num">[abc]</span></td>`).Find("td.gall_tit").First()
	if count, ok := extractCommentCount(invalidReply); count != 0 || ok {
		t.Fatalf("invalid reply comment count = %d/%v, want 0/false", count, ok)
	}
}

func TestNormalizeDCInsideURLBoundaries(t *testing.T) {
	t.Parallel()

	if got := normalizeDCInsideURL(""); got != "" {
		t.Fatalf("empty URL normalized to %q, want empty", got)
	}
	if got := normalizeDCInsideURL("http://[::1"); got != "" {
		t.Fatalf("invalid URL normalized to %q, want empty", got)
	}
	if got := normalizeDCInsideURL("javascript:alert(1)"); got != "" {
		t.Fatalf("script URL normalized to %q, want empty", got)
	}
	if got := normalizeDCInsideURL("https://example.com/mini/board/view/?id=spv&no=1"); got != "" {
		t.Fatalf("external URL normalized to %q, want empty", got)
	}
}

func TestExtractPostsWithDateRange(t *testing.T) {
	t.Parallel()

	doc := mustDocument(t, `
		<table><tbody class="listwrap2">
			<tr>
				<td class="gall_date" title="2026-07-03 12:00">12:00</td>
				<td class="gall_writer" data-uid="u1"><span class="nickname">Alice</span></td>
			</tr>
			<tr>
				<td class="gall_date" title="2026-07-02 12:00">07.02</td>
				<td class="gall_writer" data-uid="u2"><span class="nickname">Bob</span></td>
			</tr>
			<tr>
				<td class="gall_date" title="2026-06-30 12:00">06.30</td>
				<td class="gall_writer" data-uid="u3"><span class="nickname">Carol</span></td>
			</tr>
		</tbody></table>
	`)

	data := extractPostsWithDateRange(doc, "2026-07-01", "2026-07-02", time.Date(2026, 7, 3, 14, 0, 0, 0, time.Local))
	if len(data.Posts) != 1 {
		t.Fatalf("len(data.Posts) = %d, want 1", len(data.Posts))
	}
	if data.Posts[0].UID != "u2" {
		t.Fatalf("included UID = %q, want u2", data.Posts[0].UID)
	}
	if !data.FoundOlderDate {
		t.Fatal("FoundOlderDate = false, want true")
	}
}

func TestExtractPostsWithDateRangeSkipsMalformedAndOutOfRangeRows(t *testing.T) {
	t.Parallel()

	doc := mustDocument(t, `
		<table><tbody class="listwrap2">
			<tr class="ub-notice"><td class="gall_date" title="2026-07-02 12:00">07.02</td><td class="gall_writer" data-uid="notice"><span class="nickname">Notice</span></td></tr>
			<tr><td class="gall_date">2026-07-02 12:00</td><td class="gall_writer" data-uid="text-date"><span class="nickname">Text Date</span></td></tr>
			<tr><td class="gall_date" title="bad-date">bad-date</td><td class="gall_writer" data-uid="bad"><span class="nickname">Bad</span></td></tr>
			<tr><td class="gall_date" title="2026-07-04 12:00">07.04</td><td class="gall_writer" data-uid="late"><span class="nickname">Late</span></td></tr>
			<tr><td class="gall_date" title="2026-07-02 12:00">07.02</td></tr>
			<tr><td class="gall_writer" data-uid="no-date"><span class="nickname">No Date</span></td></tr>
			<tr><td class="gall_date" title="2026-07-02 12:00">07.02</td><td class="gall_writer"><span class="nickname">No UID</span></td></tr>
		</tbody></table>
	`)

	data := extractPostsWithDateRange(doc, "2026-07-01", "2026-07-03", time.Date(2026, 7, 3, 14, 0, 0, 0, time.Local))
	if len(data.Posts) != 1 {
		t.Fatalf("len(data.Posts) = %d, want 1", len(data.Posts))
	}
	if data.Posts[0].UID != "text-date" {
		t.Fatalf("included UID = %q, want text-date", data.Posts[0].UID)
	}
	if data.FoundOlderDate {
		t.Fatal("FoundOlderDate = true, want false")
	}
}

func TestAggregateAndSortUserStats(t *testing.T) {
	t.Parallel()

	posts := []Post{
		{UID: "u2", Nickname: "Bob"},
		{UID: "u1", Nickname: "Alice"},
		{UID: "u2", Nickname: "Bob"},
		{UID: "u3", Nickname: "Aaron"},
		{UID: "u4", Nickname: "Aaron"},
	}

	counts := aggregateUserPosts(posts)
	users := sortUserStats(counts)

	if len(users) != 4 {
		t.Fatalf("len(users) = %d, want 4", len(users))
	}
	if users[0].UID != "u2" || users[0].Count != 2 {
		t.Fatalf("top user = %+v, want u2 count 2", users[0])
	}
	if users[1].UID != "u3" || users[2].UID != "u4" || users[3].UID != "u1" {
		t.Fatalf("tie order = %+v, want nickname then UID ascending", users[1:])
	}
}

func TestMergeUserCountsAndFormatResult(t *testing.T) {
	t.Parallel()

	target := map[string]UserStat{
		"u1": {UID: "u1", Nickname: "Alice", Count: 2},
	}
	source := map[string]UserStat{
		"u1": {UID: "u1", Nickname: "Alice", Count: 3},
		"u2": {UID: "u2", Nickname: "Bob", IP: "(1.2)", Count: 1},
	}

	mergeUserCounts(target, source)
	if target["u1"].Count != 5 {
		t.Fatalf("merged count = %d, want 5", target["u1"].Count)
	}
	if target["u2"].IP != "(1.2)" {
		t.Fatalf("new user IP = %q, want (1.2)", target["u2"].IP)
	}

	result := formatScrapeResult(
		GalleryInfo{GalleryID: "vsoop", GalleryType: "mini", OriginalURL: "https://gall.dcinside.com/mini/vsoop"},
		scrapeTotals{UserPostCount: target, TotalPosts: 6, PagesScraped: 2},
		"2026-07-01",
		"",
	)
	if !result.Success || result.Type != "dcgallery" || result.UniqueUsers != 2 {
		t.Fatalf("unexpected result summary: %+v", result)
	}
	if result.StartDate == nil || *result.StartDate != "2026-07-01" {
		t.Fatalf("StartDate = %v, want 2026-07-01", result.StartDate)
	}
	if result.EndDate != nil {
		t.Fatalf("EndDate = %v, want nil", result.EndDate)
	}
	if len(result.TopMetrics.Views) != 0 || len(result.TopMetrics.Recommendations) != 0 || len(result.TopMetrics.Comments) != 0 {
		t.Fatalf("empty metric maps produced top metrics: %+v", result.TopMetrics)
	}
}

func TestAddPostsToTotalsBuildsTopMetricRankingsByUserBestPost(t *testing.T) {
	t.Parallel()

	totals := newScrapeTotals()
	addPostsToTotals(&totals, []Post{
		{
			UID: "u1", Nickname: "Alice", Number: "1", Title: "Alice early",
			ViewCount: 10, HasViewCount: true, RecommendCount: 1, HasRecommendCount: true, CommentCount: 1, HasCommentCount: true,
		},
		{
			UID: "u1", Nickname: "Alice", Number: "2", Title: "Alice best", URL: "https://gall.dcinside.com/mini/board/view/?id=spv&no=2",
			ViewCount: 40, HasViewCount: true, RecommendCount: 2, HasRecommendCount: true, CommentCount: 1, HasCommentCount: true,
		},
		{
			UID: "u2", Nickname: "Bob", Number: "3", Title: "Bob post",
			ViewCount: 30, HasViewCount: true, RecommendCount: 10, HasRecommendCount: true, CommentCount: 4, HasCommentCount: true,
		},
		{
			UID: "u3", Nickname: "Carol", Number: "4", Title: "Carol post",
			ViewCount: 30, HasViewCount: true, RecommendCount: 8, HasRecommendCount: true, CommentCount: 9, HasCommentCount: true,
		},
		{
			UID: "u4", Nickname: "Dave", Number: "5", Title: "Dave post",
			ViewCount: 20, HasViewCount: true, RecommendCount: 9, HasRecommendCount: true, CommentCount: 2, HasCommentCount: true,
		},
		{UID: "u5", Nickname: "Metricless"},
	})

	result := formatScrapeResult(
		GalleryInfo{GalleryID: "spv", GalleryType: "mini", OriginalURL: "https://gall.dcinside.com/mini/spv"},
		totals,
		"",
		"",
	)
	if result.TotalPosts != 6 || result.UniqueUsers != 5 {
		t.Fatalf("summary = posts %d users %d, want 6 posts and 5 users", result.TotalPosts, result.UniqueUsers)
	}
	if result.UserStats[0].UID != "u1" || result.UserStats[0].Count != 2 {
		t.Fatalf("post count ranking top = %+v, want u1 count 2", result.UserStats[0])
	}

	requireMetricOrder(t, result.TopMetrics.Views, []string{"u1", "u2", "u3"})
	if result.TopMetrics.Views[0].PostNumber != "2" || result.TopMetrics.Views[0].PostTitle != "Alice best" {
		t.Fatalf("u1 top view post = %+v, want Alice best post", result.TopMetrics.Views[0])
	}
	requireMetricOrder(t, result.TopMetrics.Recommendations, []string{"u2", "u4", "u3"})
	requireMetricOrder(t, result.TopMetrics.Comments, []string{"u3", "u2", "u4"})
}

func TestTopMetricRanksBoundariesAndTieOrder(t *testing.T) {
	t.Parallel()

	if got := topMetricRanks(nil, 3); len(got) != 0 {
		t.Fatalf("nil ranks length = %d, want 0", len(got))
	}
	if got := topMetricRanks(map[string]MetricRank{"u1": {UID: "u1", Value: 1}}, 0); len(got) != 0 {
		t.Fatalf("zero limit ranks length = %d, want 0", len(got))
	}
	if got := topMetricRanks(map[string]MetricRank{"u1": {UID: "u1", Value: 1}}, -1); len(got) != 0 {
		t.Fatalf("negative limit ranks length = %d, want 0", len(got))
	}

	ranks := topMetricRanks(map[string]MetricRank{
		"u1": {UID: "u1", Nickname: "Bob", Value: 10, PostNumber: "2"},
		"u2": {UID: "u2", Nickname: "Alice", Value: 10, PostNumber: "3"},
		"u3": {UID: "u3", Nickname: "Alice", Value: 10, PostNumber: "1"},
	}, 3)
	requireMetricOrder(t, ranks, []string{"u2", "u3", "u1"})
	for index, rank := range ranks {
		if rank.Rank != index+1 {
			t.Fatalf("rank[%d].Rank = %d, want %d", index, rank.Rank, index+1)
		}
	}
}

func TestAddPostsToTotalsHandlesNilAndIncompleteMetricPosts(t *testing.T) {
	t.Parallel()

	addPostsToTotals(nil, []Post{{UID: "ignored", HasViewCount: true, ViewCount: 99}})

	var totals scrapeTotals
	addPostsToTotals(&totals, []Post{
		{UID: "", Nickname: "No UID", ViewCount: 99, HasViewCount: true},
		{UID: "u1", Nickname: "Alice", ViewCount: 10, HasViewCount: true},
		{UID: "u1", Nickname: "Alice", ViewCount: 5, HasViewCount: true},
		{UID: "u2", Nickname: "Bob"},
	})

	if totals.TotalPosts != 4 {
		t.Fatalf("TotalPosts = %d, want 4", totals.TotalPosts)
	}
	if totals.UserPostCount == nil || totals.UserTopViews == nil || totals.UserTopRecommendations == nil || totals.UserTopComments == nil {
		t.Fatalf("totals maps were not initialized: %+v", totals)
	}
	if _, exists := totals.UserTopViews[""]; exists {
		t.Fatal("empty UID produced a top view metric")
	}
	if rank := totals.UserTopViews["u1"]; rank.Value != 10 {
		t.Fatalf("u1 top view value = %+v, want 10", rank)
	}
	if len(totals.UserTopRecommendations) != 0 || len(totals.UserTopComments) != 0 {
		t.Fatalf("metricless posts produced recommendation/comment rankings: %+v %+v", totals.UserTopRecommendations, totals.UserTopComments)
	}
	if totals.UserPostCount["u1"].Count != 2 || totals.UserPostCount["u2"].Count != 1 {
		t.Fatalf("post counts = %+v, want u1=2 and u2=1", totals.UserPostCount)
	}
}

func TestWaitBeforeRetry(t *testing.T) {
	if err := waitBeforeRetry(context.Background(), 3, 3); err != nil {
		t.Fatalf("final attempt wait returned %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitBeforeRetry(ctx, 1, 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait error = %v, want context.Canceled", err)
	}

	cappedCtx, cappedCancel := context.WithCancel(context.Background())
	cappedCancel()
	if err := waitBeforeRetry(cappedCtx, 5, 6); !errors.Is(err, context.Canceled) {
		t.Fatalf("capped cancelled wait error = %v, want context.Canceled", err)
	}

	start := time.Now()
	if err := waitBeforeRetry(context.Background(), 1, 2); err != nil {
		t.Fatalf("timer wait returned %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Fatalf("timer wait returned too early after %s", elapsed)
	}
}

func TestScrapePagesAggregatesAndEmitsProgress(t *testing.T) {
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Query().Get("page") {
		case "1":
			return htmlResponse(http.StatusOK, galleryHTML(
				postRow("u1", "Alice", "(1.1)")+
					postRow("u2", "Bob", "(2.2)"),
			)), nil
		case "2":
			return htmlResponse(http.StatusOK, galleryHTML(postRow("u1", "Alice", "(1.1)"))), nil
		default:
			t.Fatalf("unexpected page query: %s", req.URL.RawQuery)
			return nil, nil
		}
	})
	recorder := &eventRecorder{}

	result, err := scraper.Scrape(context.Background(), ScrapeRequest{
		URL:   "https://gall.dcinside.com/mini/vsoop",
		Pages: 2,
	}, recorder.emit)
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if result.PagesScraped != 2 || result.TotalPosts != 3 || result.UniqueUsers != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.UserStats[0].UID != "u1" || result.UserStats[0].Count != 2 {
		t.Fatalf("top user = %+v, want u1 count 2", result.UserStats[0])
	}
	recorder.requireEvents(t, "info", "progress", "progress", "progress", "progress", "complete")
}

func TestScrapeDefaultsToOnePageWhenPagesIsZero(t *testing.T) {
	requests := 0
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		requests++
		if got := req.URL.Query().Get("page"); got != "1" {
			t.Fatalf("page query = %q, want 1", got)
		}
		return htmlResponse(http.StatusOK, galleryHTML(postRow("u1", "Alice", ""))), nil
	})

	result, err := scraper.Scrape(context.Background(), ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop"}, nil)
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if requests != 1 || result.PagesScraped != 1 {
		t.Fatalf("requests=%d pagesScraped=%d, want 1 and 1", requests, result.PagesScraped)
	}
}

func TestScrapeRejectsNilContextAndInvalidURL(t *testing.T) {
	t.Parallel()

	scraper := NewScraper(nil)
	if _, err := scraper.Scrape(nil, ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop"}, nil); err == nil {
		t.Fatal("Scrape with nil context succeeded, want error")
	}
	if _, err := scraper.Scrape(context.Background(), ScrapeRequest{URL: "https://example.com"}, nil); err == nil {
		t.Fatal("Scrape with invalid URL succeeded, want error")
	}
}

func TestScrapeRetriesNthCallThenSucceeds(t *testing.T) {
	attempts := 0
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return htmlResponse(http.StatusInternalServerError, "server down"), nil
		}
		return htmlResponse(http.StatusOK, galleryHTML(postRow("u1", "Alice", ""))), nil
	})
	scraper.maxRetries = 2

	result, err := scraper.Scrape(context.Background(), ScrapeRequest{
		URL:   "https://gall.dcinside.com/mini/vsoop",
		Pages: 1,
	}, nil)
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if result.TotalPosts != 1 {
		t.Fatalf("TotalPosts = %d, want 1", result.TotalPosts)
	}
}

func TestScrapeContinuousPageFailureEmitsWarningAndCompletes(t *testing.T) {
	attempts := 0
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		attempts++
		return nil, errors.New("network unavailable")
	})
	scraper.maxRetries = 2
	recorder := &eventRecorder{}

	result, err := scraper.Scrape(context.Background(), ScrapeRequest{
		URL:   "https://gall.dcinside.com/mini/vsoop",
		Pages: 1,
	}, recorder.emit)
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if result.TotalPosts != 0 || result.PagesScraped != 1 {
		t.Fatalf("unexpected result after failure: %+v", result)
	}
	recorder.requireContains(t, "warning")
	recorder.requireContains(t, "complete")
}

func TestScrapePageRetryPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("temporary network error")
	})
	scraper.maxRetries = 2
	scraper.waitBeforeRetry = func(ctx context.Context, attempt int, maxAttempts int) error {
		cancel()
		return ctx.Err()
	}

	_, err := scraper.Scrape(ctx, ScrapeRequest{
		URL:   "https://gall.dcinside.com/mini/vsoop",
		Pages: 1,
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestScrapeDateRangeStopsOnOlderEmptyPage(t *testing.T) {
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Query().Get("page") {
		case "1":
			return htmlResponse(http.StatusOK, galleryHTML(datedPostRow("u1", "Alice", "2026-07-02 12:00", "07.02"))), nil
		case "2":
			return htmlResponse(http.StatusOK, galleryHTML(datedPostRow("old", "Old", "2026-06-30 12:00", "06.30"))), nil
		default:
			t.Fatalf("unexpected page query: %s", req.URL.RawQuery)
			return nil, nil
		}
	})

	result, err := scraper.Scrape(context.Background(), ScrapeRequest{
		URL:       "https://gall.dcinside.com/mini/vsoop",
		StartDate: "2026-07-01",
		EndDate:   "2026-07-03",
	}, nil)
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if result.PagesScraped != 2 || result.TotalPosts != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestScrapeDateRangeWithOnlyEndDateStopsOnNoPosts(t *testing.T) {
	requests := 0
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		requests++
		return htmlResponse(http.StatusOK, galleryHTML(datedPostRow("late", "Late", "2026-07-04 12:00", "07.04"))), nil
	})

	result, err := scraper.Scrape(context.Background(), ScrapeRequest{
		URL:     "https://gall.dcinside.com/mini/vsoop",
		EndDate: "2026-07-03",
	}, nil)
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if requests != 1 || result.PagesScraped != 1 || result.TotalPosts != 0 {
		t.Fatalf("requests=%d result=%+v, want one empty page", requests, result)
	}
}

func TestScrapeDateRangeSafetyLimit(t *testing.T) {
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		return htmlResponse(http.StatusOK, galleryHTML(datedPostRow("u1", "Alice", "2026-07-02 12:00", "07.02"))), nil
	})
	scraper.maxDateRangePages = 2

	_, err := scraper.Scrape(context.Background(), ScrapeRequest{
		URL:       "https://gall.dcinside.com/mini/vsoop",
		StartDate: "2026-07-01",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "safety limit of 2 pages") {
		t.Fatalf("error = %v, want safety limit", err)
	}
}

func TestScrapeDateRangeSkipsFailuresBeforeSafetyLimit(t *testing.T) {
	attempts := 0
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		attempts++
		return htmlResponse(http.StatusServiceUnavailable, "unavailable"), nil
	})
	scraper.maxDateRangePages = 2
	recorder := &eventRecorder{}

	_, err := scraper.Scrape(context.Background(), ScrapeRequest{
		URL:       "https://gall.dcinside.com/mini/vsoop",
		StartDate: "2026-07-01",
	}, recorder.emit)
	if err == nil || !strings.Contains(err.Error(), "safety limit of 2 pages") {
		t.Fatalf("error = %v, want safety limit", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	recorder.requireContains(t, "warning")
}

func TestScrapeDateRangeRetryPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("temporary network error")
	})
	scraper.maxRetries = 2
	scraper.waitBeforeRetry = func(ctx context.Context, attempt int, maxAttempts int) error {
		cancel()
		return ctx.Err()
	}

	_, err := scraper.Scrape(ctx, ScrapeRequest{
		URL:       "https://gall.dcinside.com/mini/vsoop",
		StartDate: "2026-07-01",
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestScrapeDateRangeRetriesNthCallThenSucceeds(t *testing.T) {
	attempts := 0
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return htmlResponse(http.StatusInternalServerError, "server down"), nil
		}
		return htmlResponse(http.StatusOK, galleryHTML(datedPostRow("u1", "Alice", "2026-06-30 12:00", "06.30"))), nil
	})
	scraper.maxRetries = 2

	result, err := scraper.Scrape(context.Background(), ScrapeRequest{
		URL:       "https://gall.dcinside.com/mini/vsoop",
		StartDate: "2026-07-01",
	}, nil)
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if result.TotalPosts != 0 || result.PagesScraped != 1 {
		t.Fatalf("result = %+v, want one older empty page", result)
	}
}

func TestScrapeDateRangeHonorsCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		t.Fatal("transport should not be called after cancellation")
		return nil, nil
	})

	_, err := scraper.Scrape(ctx, ScrapeRequest{
		URL:       "https://gall.dcinside.com/mini/vsoop",
		StartDate: "2026-07-01",
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestScrapeHonorsCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scraper := newTestScraper(func(req *http.Request) (*http.Response, error) {
		t.Fatal("transport should not be called after cancellation")
		return nil, nil
	})

	_, err := scraper.Scrape(ctx, ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop", Pages: 1}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestFetchDocumentStatusHandlingAndResourceCleanup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantError  string
	}{
		{name: "success", statusCode: http.StatusOK},
		{name: "client error", statusCode: http.StatusNotFound, wantError: "request failed: 404"},
		{name: "server error", statusCode: http.StatusBadGateway, wantError: "server error: 502"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bodyClosed := false
			scraper := NewScraper(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get("User-Agent") == "" {
					t.Fatal("User-Agent header is empty")
				}
				if req.Header.Get("Accept-Language") == "" {
					t.Fatal("Accept-Language header is empty")
				}
				return &http.Response{
					StatusCode: test.statusCode,
					Body:       &trackingReadCloser{reader: strings.NewReader(galleryHTML("")), closed: &bodyClosed},
					Header:     make(http.Header),
					Request:    req,
				}, nil
			})})

			_, err := scraper.fetchDocument(context.Background(), "https://gall.dcinside.com/mini/board/lists/?id=vsoop&page=1")
			if test.wantError == "" && err != nil {
				t.Fatalf("fetchDocument returned error: %v", err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
			if !bodyClosed {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestFetchDocumentPropagatesRequestAndTransportErrors(t *testing.T) {
	t.Parallel()

	scraper := NewScraper(nil)
	if _, err := scraper.fetchDocument(context.Background(), "http://[::1"); err == nil {
		t.Fatal("invalid request URL succeeded, want error")
	}

	scraper = NewScraper(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("dial failed")
	})})
	if _, err := scraper.fetchDocument(context.Background(), "https://gall.dcinside.com/mini/board/lists/?id=vsoop&page=1"); err == nil {
		t.Fatal("transport error was nil, want error")
	}
}

func TestFetchDocumentPropagatesReadErrorsAndClosesBody(t *testing.T) {
	t.Parallel()

	bodyClosed := false
	scraper := NewScraper(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       &errorReadCloser{closed: &bodyClosed},
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})})

	_, err := scraper.fetchDocument(context.Background(), "https://gall.dcinside.com/mini/board/lists/?id=vsoop&page=1")
	if err == nil || !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("error = %v, want read failed", err)
	}
	if !bodyClosed {
		t.Fatal("response body was not closed")
	}
}

func TestFetchDocumentUsesDefaultClientWhenScraperClientIsNil(t *testing.T) {
	originalDefaultClient := http.DefaultClient
	t.Cleanup(func() {
		http.DefaultClient = originalDefaultClient
	})

	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return htmlResponse(http.StatusOK, galleryHTML("")), nil
	})}

	scraper := &Scraper{}
	if _, err := scraper.fetchDocument(context.Background(), "https://gall.dcinside.com/mini/board/lists/?id=vsoop&page=1"); err != nil {
		t.Fatalf("fetchDocument returned error: %v", err)
	}
}

func TestScrapeUsesDefaultFallbacksForZeroInternalLimits(t *testing.T) {
	requests := 0
	scraper := NewScraper(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return htmlResponse(http.StatusOK, galleryHTML(postRow("u1", "Alice", ""))), nil
	})})
	scraper.maxRetries = 0
	scraper.maxDateRangePages = 0
	scraper.waitBeforeRetry = noRetryWait

	result, err := scraper.Scrape(context.Background(), ScrapeRequest{URL: "https://gall.dcinside.com/mini/vsoop", Pages: 1}, nil)
	if err != nil {
		t.Fatalf("Scrape returned error: %v", err)
	}
	if requests != 1 || result.TotalPosts != 1 {
		t.Fatalf("requests=%d TotalPosts=%d, want 1 and 1", requests, result.TotalPosts)
	}
}

func TestInternalDefaultHelpers(t *testing.T) {
	t.Parallel()

	var nilScraper *Scraper
	if nilScraper.retryLimit() != defaultMaxRetries {
		t.Fatalf("nil retryLimit = %d, want default", nilScraper.retryLimit())
	}
	if nilScraper.dateRangePageLimit() != defaultMaxDateRangePages {
		t.Fatalf("nil dateRangePageLimit = %d, want default", nilScraper.dateRangePageLimit())
	}
	if nilScraper.currentTime().IsZero() {
		t.Fatal("nil currentTime returned zero time")
	}
	if err := nilScraper.wait(context.Background(), 1, 1); err != nil {
		t.Fatalf("nil wait final attempt returned %v, want nil", err)
	}

	scraper := &Scraper{}
	if scraper.retryLimit() != defaultMaxRetries {
		t.Fatalf("zero retryLimit = %d, want default", scraper.retryLimit())
	}
	if scraper.dateRangePageLimit() != defaultMaxDateRangePages {
		t.Fatalf("zero dateRangePageLimit = %d, want default", scraper.dateRangePageLimit())
	}
	if scraper.currentTime().IsZero() {
		t.Fatal("zero currentTime returned zero time")
	}
}

func TestDateDigitHelpers(t *testing.T) {
	t.Parallel()

	if !isTwoDigits("09") {
		t.Fatal("isTwoDigits rejected 09")
	}
	for _, value := range []string{"", "9", "999", "0x", "x0"} {
		if isTwoDigits(value) {
			t.Fatalf("isTwoDigits(%q) = true, want false", value)
		}
	}
}

func TestExtractPostFromEmptyWriter(t *testing.T) {
	t.Parallel()

	empty := mustDocument(t, `<table></table>`).Find("td.gall_writer").First()
	if post, ok := extractPostFromWriter(empty); ok || post.UID != "" {
		t.Fatalf("extractPostFromWriter = %+v, %v; want empty false", post, ok)
	}
}

func TestEmitEventAllowsNilEmitter(t *testing.T) {
	t.Parallel()
	emitEvent(nil, "progress", ProgressInfo{})
}

func mustDocument(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("NewDocumentFromReader returned error: %v", err)
	}
	return doc
}

func newTestScraper(fn roundTripFunc) *Scraper {
	scraper := NewScraper(&http.Client{Transport: fn})
	scraper.maxRetries = 1
	scraper.maxDateRangePages = 10
	scraper.waitBeforeRetry = noRetryWait
	scraper.now = func() time.Time {
		return time.Date(2026, 7, 3, 14, 0, 0, 0, time.Local)
	}
	return scraper
}

func noRetryWait(ctx context.Context, _ int, _ int) error {
	return ctx.Err()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func htmlResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func galleryHTML(rows string) string {
	return `<table><tbody class="listwrap2">` + rows + `</tbody></table>`
}

func postRow(uid string, nickname string, ip string) string {
	return `<tr><td class="gall_writer" data-uid="` + uid + `"><span class="nickname">` + nickname + `</span><span class="ip">` + ip + `</span></td></tr>`
}

func metricPostRow(number string, uid string, nickname string, ip string, title string, href string, views string, recommends string, replies string) string {
	replyHTML := ""
	if replies != "" {
		replyHTML = `<a class="reply_numbox" href="` + href + `"><span class="reply_num">` + replies + `</span></a>`
	}
	return `<tr>` +
		`<td class="gall_num">` + number + `</td>` +
		`<td class="gall_tit ub-word"><a href="` + href + `">` + title + `</a>` + replyHTML + `</td>` +
		`<td class="gall_writer ub-writer" data-nick="` + nickname + `" data-uid="` + uid + `" data-ip="` + ip + `"></td>` +
		`<td class="gall_count">` + views + `</td>` +
		`<td class="gall_recommend">` + recommends + `</td>` +
		`</tr>`
}

func datedPostRow(uid string, nickname string, dateTitle string, dateText string) string {
	return `<tr><td class="gall_date" title="` + dateTitle + `">` + dateText + `</td><td class="gall_writer" data-uid="` + uid + `"><span class="nickname">` + nickname + `</span></td></tr>`
}

type trackingReadCloser struct {
	reader *strings.Reader
	closed *bool
}

type errorReadCloser struct {
	closed *bool
}

func (e *errorReadCloser) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

func (e *errorReadCloser) Close() error {
	*e.closed = true
	return nil
}

func (t *trackingReadCloser) Read(p []byte) (int, error) {
	return t.reader.Read(p)
}

func (t *trackingReadCloser) Close() error {
	*t.closed = true
	return nil
}

type recordedEvent struct {
	name    string
	payload interface{}
}

type eventRecorder struct {
	events []recordedEvent
}

func (r *eventRecorder) emit(eventName string, payload interface{}) {
	r.events = append(r.events, recordedEvent{name: eventName, payload: payload})
}

func (r *eventRecorder) requireEvents(t *testing.T, names ...string) {
	t.Helper()
	if len(r.events) != len(names) {
		t.Fatalf("event count = %d, want %d: %+v", len(r.events), len(names), r.events)
	}
	for i, name := range names {
		if r.events[i].name != name {
			t.Fatalf("event[%d] = %q, want %q", i, r.events[i].name, name)
		}
	}
}

func (r *eventRecorder) requireContains(t *testing.T, name string) {
	t.Helper()
	for _, event := range r.events {
		if event.name == name {
			return
		}
	}
	t.Fatalf("events do not contain %q: %+v", name, r.events)
}

func requireMetricOrder(t *testing.T, ranks []MetricRank, wantUIDs []string) {
	t.Helper()
	if len(ranks) != len(wantUIDs) {
		t.Fatalf("metric rank length = %d, want %d: %+v", len(ranks), len(wantUIDs), ranks)
	}
	for index, wantUID := range wantUIDs {
		if ranks[index].UID != wantUID {
			t.Fatalf("rank[%d].UID = %q, want %q: %+v", index, ranks[index].UID, wantUID, ranks)
		}
	}
}
