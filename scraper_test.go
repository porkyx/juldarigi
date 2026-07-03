package main

import (
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

	for _, rawURL := range []string{"", "https://example.com/mini/vsoop", "https://gall.dcinside.com/mini/board/lists/"} {
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

	info := GalleryInfo{GalleryID: "hello world", GalleryType: "mini"}
	got := BuildPageURL(info, 3)
	want := "https://gall.dcinside.com/mini/board/lists/?id=hello+world&page=3"
	if got != want {
		t.Fatalf("BuildPageURL() = %q, want %q", got, want)
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
			<tr><td class="gall_writer"><span class="nickname">No UID</span></td></tr>
		</tbody></table>
	`)

	posts := extractPostsFromDocument(doc)
	if len(posts) != 2 {
		t.Fatalf("len(posts) = %d, want 2", len(posts))
	}
	if posts[0].UID != "u1" || posts[0].Nickname != "Alice" || posts[0].IP != "(1.2)" {
		t.Fatalf("first post = %+v", posts[0])
	}
	if posts[1].UID != "u2" || posts[1].Nickname != "Bob" {
		t.Fatalf("second post = %+v", posts[1])
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

func TestAggregateAndSortUserStats(t *testing.T) {
	t.Parallel()

	posts := []Post{
		{UID: "u2", Nickname: "Bob"},
		{UID: "u1", Nickname: "Alice"},
		{UID: "u2", Nickname: "Bob"},
		{UID: "u3", Nickname: "Aaron"},
	}

	counts := aggregateUserPosts(posts)
	users := sortUserStats(counts)

	if len(users) != 3 {
		t.Fatalf("len(users) = %d, want 3", len(users))
	}
	if users[0].UID != "u2" || users[0].Count != 2 {
		t.Fatalf("top user = %+v, want u2 count 2", users[0])
	}
	if users[1].UID != "u3" || users[2].UID != "u1" {
		t.Fatalf("tie order = %+v, want nickname ascending", users[1:])
	}
}

func mustDocument(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("NewDocumentFromReader returned error: %v", err)
	}
	return doc
}
