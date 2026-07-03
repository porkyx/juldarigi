package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	defaultPages             = 1
	defaultMaxRetries        = 5
	defaultMaxDateRangePages = 10000
)

type EventEmitter func(eventName string, payload interface{})
type retryWaiter func(ctx context.Context, attempt int, maxAttempts int) error

type ScrapeRequest struct {
	URL       string `json:"url"`
	Pages     int    `json:"pages"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type ScrapeResult struct {
	Success      bool           `json:"success"`
	Type         string         `json:"type"`
	GalleryID    string         `json:"galleryId"`
	GalleryType  string         `json:"galleryType"`
	URL          string         `json:"url"`
	PagesScraped int            `json:"pagesScraped"`
	StartDate    *string        `json:"startDate"`
	EndDate      *string        `json:"endDate"`
	TotalPosts   int            `json:"totalPosts"`
	UniqueUsers  int            `json:"uniqueUsers"`
	UserStats    []UserStat     `json:"userStats"`
	TopMetrics   MetricRankings `json:"topMetrics"`
}

type UserStat struct {
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
	IP       string `json:"ip"`
	Count    int    `json:"count"`
}

type MetricRankings struct {
	Views           []MetricRank `json:"views"`
	Recommendations []MetricRank `json:"recommendations"`
	Comments        []MetricRank `json:"comments"`
}

type MetricRank struct {
	Rank       int    `json:"rank"`
	UID        string `json:"uid"`
	Nickname   string `json:"nickname"`
	IP         string `json:"ip"`
	Value      int    `json:"value"`
	PostNumber string `json:"postNumber"`
	PostTitle  string `json:"postTitle"`
	PostURL    string `json:"postUrl"`
}

type ProgressInfo struct {
	CurrentPage int    `json:"currentPage"`
	TotalPages  int    `json:"totalPages,omitempty"`
	TotalPosts  int    `json:"totalPosts"`
	UniqueUsers int    `json:"uniqueUsers"`
	Message     string `json:"message"`
}

type MessagePayload struct {
	Message string `json:"message"`
}

type GalleryInfo struct {
	GalleryID   string
	GalleryType string
	OriginalURL string
}

type Post struct {
	UID               string
	Nickname          string
	IP                string
	Date              string
	Number            string
	Title             string
	URL               string
	ViewCount         int
	HasViewCount      bool
	RecommendCount    int
	HasRecommendCount bool
	CommentCount      int
	HasCommentCount   bool
}

type datePageData struct {
	Posts          []Post
	FoundOlderDate bool
}

type scrapeTotals struct {
	UserPostCount          map[string]UserStat
	UserTopViews           map[string]MetricRank
	UserTopRecommendations map[string]MetricRank
	UserTopComments        map[string]MetricRank
	TotalPosts             int
	PagesScraped           int
}

type Scraper struct {
	client            *http.Client
	now               func() time.Time
	waitBeforeRetry   retryWaiter
	maxRetries        int
	maxDateRangePages int
}

func NewScraper(client *http.Client) *Scraper {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	return &Scraper{
		client:            client,
		now:               time.Now,
		waitBeforeRetry:   waitBeforeRetry,
		maxRetries:        defaultMaxRetries,
		maxDateRangePages: defaultMaxDateRangePages,
	}
}

func (s *Scraper) Scrape(ctx context.Context, request ScrapeRequest, emit EventEmitter) (*ScrapeResult, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	galleryInfo, err := ParseGalleryURL(request.URL)
	if err != nil {
		return nil, err
	}

	var totals scrapeTotals
	if request.StartDate != "" || request.EndDate != "" {
		totals, err = s.scrapeDateRange(ctx, galleryInfo, request.StartDate, request.EndDate, emit)
	} else {
		pages := request.Pages
		if pages < 1 {
			pages = defaultPages
		}
		totals, err = s.scrapePages(ctx, galleryInfo, pages, emit)
	}
	if err != nil {
		return nil, err
	}

	result := formatScrapeResult(galleryInfo, totals, request.StartDate, request.EndDate)
	emitEvent(emit, "complete", result)
	return result, nil
}

func ParseGalleryURL(rawURL string) (GalleryInfo, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return GalleryInfo{}, errors.New("URL is required")
	}

	parseTarget := trimmed
	if !strings.HasPrefix(parseTarget, "http://") && !strings.HasPrefix(parseTarget, "https://") {
		parseTarget = "https://" + parseTarget
	}

	parsed, err := url.Parse(parseTarget)
	if err != nil {
		return GalleryInfo{}, errors.New("invalid URL format")
	}
	if parsed.Hostname() != "gall.dcinside.com" {
		return GalleryInfo{}, errors.New("Invalid URL format. Expected DCInside gallery URL")
	}

	segments := splitPath(parsed.Path)
	queryID := strings.TrimSpace(parsed.Query().Get("id"))

	switch {
	case len(segments) >= 3 && segments[0] == "mini" && segments[1] == "board" && segments[2] == "lists":
		return galleryInfoFromID(trimmed, queryID, "mini")
	case len(segments) >= 2 && segments[0] == "mini" && segments[1] == "board":
		return GalleryInfo{}, errors.New("Invalid URL format. Expected DCInside gallery URL")
	case len(segments) >= 2 && segments[0] == "mini":
		return galleryInfoFromID(trimmed, segments[1], "mini")
	case len(segments) >= 3 && segments[0] == "mgallery" && segments[1] == "board" && segments[2] == "lists":
		return galleryInfoFromID(trimmed, queryID, "mgallery")
	case len(segments) >= 2 && segments[0] == "mgallery" && segments[1] == "board":
		return GalleryInfo{}, errors.New("Invalid URL format. Expected DCInside gallery URL")
	case len(segments) >= 2 && segments[0] == "mgallery":
		return galleryInfoFromID(trimmed, segments[1], "mgallery")
	case len(segments) >= 2 && segments[0] == "board" && segments[1] == "lists":
		return galleryInfoFromID(trimmed, queryID, "board")
	case len(segments) == 1:
		return galleryInfoFromID(trimmed, segments[0], "board")
	default:
		return GalleryInfo{}, errors.New("Invalid URL format. Expected DCInside gallery URL")
	}
}

func BuildPageURL(galleryInfo GalleryInfo, pageNumber int) string {
	switch galleryInfo.GalleryType {
	case "mgallery":
		return fmt.Sprintf("https://gall.dcinside.com/mgallery/board/lists/?id=%s&page=%d", url.QueryEscape(galleryInfo.GalleryID), pageNumber)
	case "mini":
		return fmt.Sprintf("https://gall.dcinside.com/mini/board/lists/?id=%s&page=%d", url.QueryEscape(galleryInfo.GalleryID), pageNumber)
	default:
		return fmt.Sprintf("https://gall.dcinside.com/board/lists/?id=%s&page=%d", url.QueryEscape(galleryInfo.GalleryID), pageNumber)
	}
}

func splitPath(path string) []string {
	rawSegments := strings.Split(strings.Trim(path, "/"), "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func galleryInfoFromID(originalURL, galleryID, galleryType string) (GalleryInfo, error) {
	galleryID = strings.TrimSpace(galleryID)
	if galleryID == "" {
		return GalleryInfo{}, errors.New("Invalid URL format. Expected DCInside gallery URL")
	}

	return GalleryInfo{
		GalleryID:   galleryID,
		GalleryType: galleryType,
		OriginalURL: originalURL,
	}, nil
}

func (s *Scraper) scrapePages(ctx context.Context, galleryInfo GalleryInfo, pages int, emit EventEmitter) (scrapeTotals, error) {
	totals := newScrapeTotals()
	emitEvent(emit, "info", MessagePayload{Message: fmt.Sprintf("Starting page-based scraping for %d pages", pages)})

	for pageNumber := 1; pageNumber <= pages; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return scrapeTotals{}, err
		}

		emitEvent(emit, "progress", ProgressInfo{
			CurrentPage: pageNumber,
			TotalPages:  pages,
			TotalPosts:  totals.TotalPosts,
			UniqueUsers: len(totals.UserPostCount),
			Message:     fmt.Sprintf("Scraping page %d of %d...", pageNumber, pages),
		})

		pageURL := BuildPageURL(galleryInfo, pageNumber)
		posts, err := s.scrapePostsWithRetry(ctx, pageURL)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return scrapeTotals{}, ctxErr
			}
			emitEvent(emit, "warning", MessagePayload{Message: fmt.Sprintf("Skipping page %d due to errors: %s", pageNumber, err.Error())})
			posts = nil
		}

		addPostsToTotals(&totals, posts)
		totals.PagesScraped = pageNumber

		emitEvent(emit, "progress", ProgressInfo{
			CurrentPage: pageNumber,
			TotalPages:  pages,
			TotalPosts:  totals.TotalPosts,
			UniqueUsers: len(totals.UserPostCount),
			Message:     fmt.Sprintf("Page %d completed: %d posts found", pageNumber, len(posts)),
		})
	}

	return totals, nil
}

func (s *Scraper) scrapeDateRange(ctx context.Context, galleryInfo GalleryInfo, startDate, endDate string, emit EventEmitter) (scrapeTotals, error) {
	totals := newScrapeTotals()
	emitEvent(emit, "info", MessagePayload{Message: fmt.Sprintf("Starting date range scraping from %s to %s", displayDateBoundary(startDate, "beginning"), displayDateBoundary(endDate, "latest"))})

	pageLimit := s.dateRangePageLimit()
	for pageNumber := 1; pageNumber <= pageLimit; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return scrapeTotals{}, err
		}

		emitEvent(emit, "progress", ProgressInfo{
			CurrentPage: pageNumber,
			TotalPosts:  totals.TotalPosts,
			UniqueUsers: len(totals.UserPostCount),
			Message:     fmt.Sprintf("Scraping page %d...", pageNumber),
		})

		pageURL := BuildPageURL(galleryInfo, pageNumber)
		pageData, err := s.scrapeDatePageWithRetry(ctx, pageURL, startDate, endDate)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return scrapeTotals{}, ctxErr
			}
			emitEvent(emit, "warning", MessagePayload{Message: fmt.Sprintf("Skipping page %d due to errors: %s", pageNumber, err.Error())})
			totals.PagesScraped = pageNumber
			continue
		}

		addPostsToTotals(&totals, pageData.Posts)
		totals.PagesScraped = pageNumber

		emitEvent(emit, "progress", ProgressInfo{
			CurrentPage: pageNumber,
			TotalPosts:  totals.TotalPosts,
			UniqueUsers: len(totals.UserPostCount),
			Message:     fmt.Sprintf("Page %d completed: %d posts found", pageNumber, len(pageData.Posts)),
		})

		if startDate != "" && pageData.FoundOlderDate && len(pageData.Posts) == 0 {
			emitEvent(emit, "info", MessagePayload{Message: "Found posts older than target date, stopping..."})
			return totals, nil
		}
		if startDate == "" && len(pageData.Posts) == 0 {
			emitEvent(emit, "info", MessagePayload{Message: "No more posts found, stopping..."})
			return totals, nil
		}
	}

	return scrapeTotals{}, fmt.Errorf("date range scraping stopped after safety limit of %d pages", pageLimit)
}

func (s *Scraper) scrapePostsWithRetry(ctx context.Context, pageURL string) ([]Post, error) {
	var lastErr error
	retryLimit := s.retryLimit()
	for attempt := 1; attempt <= retryLimit; attempt++ {
		doc, err := s.fetchDocument(ctx, pageURL)
		if err == nil {
			return extractPostsFromDocument(doc), nil
		}
		lastErr = err
		if err := s.wait(ctx, attempt, retryLimit); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (s *Scraper) scrapeDatePageWithRetry(ctx context.Context, pageURL, startDate, endDate string) (datePageData, error) {
	var lastErr error
	retryLimit := s.retryLimit()
	for attempt := 1; attempt <= retryLimit; attempt++ {
		doc, err := s.fetchDocument(ctx, pageURL)
		if err == nil {
			return extractPostsWithDateRange(doc, startDate, endDate, s.currentTime()), nil
		}
		lastErr = err
		if err := s.wait(ctx, attempt, retryLimit); err != nil {
			return datePageData{}, err
		}
	}
	return datePageData{}, lastErr
}

func (s *Scraper) fetchDocument(ctx context.Context, pageURL string) (*goquery.Document, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36")
	request.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,en;q=0.8")

	client := s.client
	if client == nil {
		client = http.DefaultClient
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode >= 500 {
		return nil, fmt.Errorf("server error: %d", response.StatusCode)
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed: %d", response.StatusCode)
	}

	return goquery.NewDocumentFromReader(response.Body)
}

func extractPostsFromDocument(doc *goquery.Document) []Post {
	posts := make([]Post, 0)
	doc.Find("tbody.listwrap2 tr").Each(func(_ int, row *goquery.Selection) {
		post, ok := extractPost(row)
		if ok {
			posts = append(posts, post)
		}
	})
	return posts
}

func extractPostsWithDateRange(doc *goquery.Document, startDate, endDate string, now time.Time) datePageData {
	data := datePageData{Posts: make([]Post, 0)}
	doc.Find("tbody.listwrap2 tr").Each(func(_ int, row *goquery.Selection) {
		if isNoticeRow(row) {
			return
		}

		dateSelection := row.Find("td.gall_date").First()
		writerSelection := row.Find("td.gall_writer").First()
		if dateSelection.Length() == 0 || writerSelection.Length() == 0 {
			return
		}

		postDate := strings.TrimSpace(selectionAttrOrText(dateSelection, "title"))
		normalizedDate := normalizeDateWithNow(postDate, now)
		if normalizedDate == "" {
			return
		}

		if startDate != "" && normalizedDate < startDate {
			data.FoundOlderDate = true
			return
		}
		if endDate != "" && normalizedDate > endDate {
			return
		}

		post, ok := extractPostFromWriter(writerSelection)
		if !ok {
			return
		}
		post.Date = postDate
		enrichPostFromRow(row, &post)
		data.Posts = append(data.Posts, post)
	})

	return data
}

func extractPost(row *goquery.Selection) (Post, bool) {
	if isNoticeRow(row) {
		return Post{}, false
	}
	post, ok := extractPostFromWriter(row.Find("td.gall_writer").First())
	if !ok {
		return Post{}, false
	}
	enrichPostFromRow(row, &post)
	return post, true
}

func extractPostFromWriter(writerSelection *goquery.Selection) (Post, bool) {
	if writerSelection.Length() == 0 {
		return Post{}, false
	}

	uid, exists := writerSelection.Attr("data-uid")
	uid = strings.TrimSpace(uid)
	if !exists || uid == "" {
		return Post{}, false
	}

	nickname := strings.TrimSpace(writerSelection.Find(".nickname").First().Text())
	if nickname == "" {
		nickname = strings.TrimSpace(writerSelection.Find(".nick_comm").First().Text())
	}
	if nickname == "" {
		nickname = strings.TrimSpace(selectionAttr(writerSelection, "data-nick"))
	}
	if nickname == "" {
		nickname = "Unknown"
	}

	ip := strings.TrimSpace(writerSelection.Find(".ip").First().Text())
	if ip == "" {
		ip = strings.TrimSpace(selectionAttr(writerSelection, "data-ip"))
	}

	return Post{
		UID:      uid,
		Nickname: nickname,
		IP:       ip,
	}, true
}

func enrichPostFromRow(row *goquery.Selection, post *Post) {
	if row == nil || post == nil {
		return
	}

	post.Number = strings.TrimSpace(row.Find("td.gall_num").First().Text())

	titleSelection := row.Find("td.gall_tit").First()
	if titleSelection.Length() > 0 {
		post.CommentCount, post.HasCommentCount = extractCommentCount(titleSelection)
		titleLink := titleSelection.Find("a").Not(".reply_numbox").First()
		if titleLink.Length() == 0 {
			titleLink = titleSelection.Find(`a[href*="/board/view/"]`).First()
		}
		if titleLink.Length() > 0 {
			post.Title = normalizeWhitespace(titleLink.Text())
			post.URL = normalizeDCInsideURL(selectionAttr(titleLink, "href"))
		} else {
			post.Title = normalizeWhitespace(titleSelection.Text())
		}
	}

	if value, ok := extractMetricFromSelection(row.Find("td.gall_count").First()); ok {
		post.ViewCount = value
		post.HasViewCount = true
	}
	if value, ok := extractMetricFromSelection(row.Find("td.gall_recommend").First()); ok {
		post.RecommendCount = value
		post.HasRecommendCount = true
	}
}

func isNoticeRow(row *goquery.Selection) bool {
	return row.HasClass("ub-notice") || row.Find("em.icon_img.icon_notice").Length() > 0
}

func selectionAttrOrText(selection *goquery.Selection, attrName string) string {
	value, exists := selection.Attr(attrName)
	if exists && strings.TrimSpace(value) != "" {
		return value
	}
	return selection.Text()
}

func selectionAttr(selection *goquery.Selection, attrName string) string {
	value, exists := selection.Attr(attrName)
	if !exists {
		return ""
	}
	return value
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func normalizeDCInsideURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	base, _ := url.Parse("https://gall.dcinside.com")
	resolved := base.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	if resolved.Hostname() != "gall.dcinside.com" {
		return ""
	}
	return resolved.String()
}

func extractMetricFromSelection(selection *goquery.Selection) (int, bool) {
	if selection.Length() == 0 {
		return 0, false
	}
	return parseMetricText(selection.Text())
}

func extractCommentCount(titleSelection *goquery.Selection) (int, bool) {
	if titleSelection.Length() == 0 {
		return 0, false
	}

	replySelection := titleSelection.Find(".reply_num").First()
	if replySelection.Length() == 0 {
		return 0, true
	}
	return parseMetricText(replySelection.Text())
}

func parseMetricText(value string) (int, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "-" {
		return 0, false
	}

	var digits strings.Builder
	for _, char := range trimmed {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		}
	}
	if digits.Len() == 0 {
		return 0, false
	}

	metric, err := strconv.Atoi(digits.String())
	if err != nil {
		return 0, false
	}
	return metric, true
}

func normalizeDateWithNow(dateString string, now time.Time) string {
	trimmed := strings.TrimSpace(dateString)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) >= len("2006-01-02") && isDatePrefix(trimmed[:len("2006-01-02")]) {
		return trimmed[:len("2006-01-02")]
	}
	if len(trimmed) == len("15:04") && trimmed[2] == ':' && isTwoDigits(trimmed[:2]) && isTwoDigits(trimmed[3:]) {
		return now.Format("2006-01-02")
	}
	if len(trimmed) == len("01.02") && trimmed[2] == '.' && isTwoDigits(trimmed[:2]) && isTwoDigits(trimmed[3:]) {
		return fmt.Sprintf("%04d-%s-%s", now.Year(), trimmed[:2], trimmed[3:])
	}
	return ""
}

func isTwoDigits(value string) bool {
	if len(value) != 2 {
		return false
	}
	return value[0] >= '0' && value[0] <= '9' && value[1] >= '0' && value[1] <= '9'
}

func isDatePrefix(value string) bool {
	for i, char := range value {
		switch i {
		case 4, 7:
			if char != '-' {
				return false
			}
		default:
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func aggregateUserPosts(posts []Post) map[string]UserStat {
	counts := make(map[string]UserStat)
	for _, post := range posts {
		user := counts[post.UID]
		if user.UID == "" {
			user = UserStat{
				UID:      post.UID,
				Nickname: post.Nickname,
				IP:       post.IP,
			}
		}
		user.Count++
		counts[post.UID] = user
	}
	return counts
}

func newScrapeTotals() scrapeTotals {
	return scrapeTotals{
		UserPostCount:          map[string]UserStat{},
		UserTopViews:           map[string]MetricRank{},
		UserTopRecommendations: map[string]MetricRank{},
		UserTopComments:        map[string]MetricRank{},
	}
}

func addPostsToTotals(totals *scrapeTotals, posts []Post) {
	if totals == nil {
		return
	}
	ensureTotalsMaps(totals)
	mergeUserCounts(totals.UserPostCount, aggregateUserPosts(posts))
	mergeUserMetricBests(totals.UserTopViews, posts, func(post Post) (int, bool) {
		return post.ViewCount, post.HasViewCount
	})
	mergeUserMetricBests(totals.UserTopRecommendations, posts, func(post Post) (int, bool) {
		return post.RecommendCount, post.HasRecommendCount
	})
	mergeUserMetricBests(totals.UserTopComments, posts, func(post Post) (int, bool) {
		return post.CommentCount, post.HasCommentCount
	})
	totals.TotalPosts += len(posts)
}

func ensureTotalsMaps(totals *scrapeTotals) {
	if totals.UserPostCount == nil {
		totals.UserPostCount = map[string]UserStat{}
	}
	if totals.UserTopViews == nil {
		totals.UserTopViews = map[string]MetricRank{}
	}
	if totals.UserTopRecommendations == nil {
		totals.UserTopRecommendations = map[string]MetricRank{}
	}
	if totals.UserTopComments == nil {
		totals.UserTopComments = map[string]MetricRank{}
	}
}

func mergeUserCounts(target map[string]UserStat, source map[string]UserStat) {
	for uid, sourceUser := range source {
		targetUser := target[uid]
		if targetUser.UID == "" {
			target[uid] = sourceUser
			continue
		}
		targetUser.Count += sourceUser.Count
		target[uid] = targetUser
	}
}

func mergeUserMetricBests(target map[string]MetricRank, posts []Post, valueFor func(Post) (int, bool)) {
	for _, post := range posts {
		if post.UID == "" {
			continue
		}
		value, ok := valueFor(post)
		if !ok {
			continue
		}

		candidate := metricRankFromPost(post, value)
		current, exists := target[post.UID]
		if !exists || candidate.Value > current.Value {
			target[post.UID] = candidate
		}
	}
}

func metricRankFromPost(post Post, value int) MetricRank {
	return MetricRank{
		UID:        post.UID,
		Nickname:   post.Nickname,
		IP:         post.IP,
		Value:      value,
		PostNumber: post.Number,
		PostTitle:  post.Title,
		PostURL:    post.URL,
	}
}

func sortUserStats(userPostCount map[string]UserStat) []UserStat {
	users := make([]UserStat, 0, len(userPostCount))
	for _, user := range userPostCount {
		users = append(users, user)
	}

	sort.Slice(users, func(i, j int) bool {
		if users[i].Count != users[j].Count {
			return users[i].Count > users[j].Count
		}
		if users[i].Nickname != users[j].Nickname {
			return users[i].Nickname < users[j].Nickname
		}
		return users[i].UID < users[j].UID
	})

	return users
}

func topMetricRanks(userMetricBests map[string]MetricRank, limit int) []MetricRank {
	if limit < 1 {
		return []MetricRank{}
	}

	ranks := make([]MetricRank, 0, len(userMetricBests))
	for _, rank := range userMetricBests {
		ranks = append(ranks, rank)
	}

	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].Value != ranks[j].Value {
			return ranks[i].Value > ranks[j].Value
		}
		if ranks[i].Nickname != ranks[j].Nickname {
			return ranks[i].Nickname < ranks[j].Nickname
		}
		return ranks[i].UID < ranks[j].UID
	})

	if limit < len(ranks) {
		ranks = ranks[:limit]
	}
	for index := range ranks {
		ranks[index].Rank = index + 1
	}
	return ranks
}

func formatScrapeResult(galleryInfo GalleryInfo, totals scrapeTotals, startDate, endDate string) *ScrapeResult {
	userStats := sortUserStats(totals.UserPostCount)
	return &ScrapeResult{
		Success:      true,
		Type:         "dcgallery",
		GalleryID:    galleryInfo.GalleryID,
		GalleryType:  galleryInfo.GalleryType,
		URL:          galleryInfo.OriginalURL,
		PagesScraped: totals.PagesScraped,
		StartDate:    optionalString(startDate),
		EndDate:      optionalString(endDate),
		TotalPosts:   totals.TotalPosts,
		UniqueUsers:  len(userStats),
		UserStats:    userStats,
		TopMetrics: MetricRankings{
			Views:           topMetricRanks(totals.UserTopViews, 3),
			Recommendations: topMetricRanks(totals.UserTopRecommendations, 3),
			Comments:        topMetricRanks(totals.UserTopComments, 3),
		},
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func displayDateBoundary(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (s *Scraper) retryLimit() int {
	if s == nil || s.maxRetries < 1 {
		return defaultMaxRetries
	}
	return s.maxRetries
}

func (s *Scraper) dateRangePageLimit() int {
	if s == nil || s.maxDateRangePages < 1 {
		return defaultMaxDateRangePages
	}
	return s.maxDateRangePages
}

func (s *Scraper) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now()
	}
	return s.now()
}

func (s *Scraper) wait(ctx context.Context, attempt int, maxAttempts int) error {
	if s != nil && s.waitBeforeRetry != nil {
		return s.waitBeforeRetry(ctx, attempt, maxAttempts)
	}
	return waitBeforeRetry(ctx, attempt, maxAttempts)
}

func waitBeforeRetry(ctx context.Context, attempt int, maxAttempts int) error {
	if attempt >= maxAttempts {
		return nil
	}

	backoff := time.Duration(1<<(attempt-1)) * time.Second
	if backoff > 10*time.Second {
		backoff = 10 * time.Second
	}

	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func emitEvent(emit EventEmitter, eventName string, payload interface{}) {
	if emit != nil {
		emit(eventName, payload)
	}
}
