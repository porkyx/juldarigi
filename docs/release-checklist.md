# Release Candidate Checklist

Use this checklist for every release candidate before tagging or distributing a build.

## User-visible Behavior

- Verify page-count scraping with 1 page, multiple pages, and a page that returns no posts.
- Verify date-range scraping with start date only, end date only, both dates, and an empty result.
- Verify invalid DCInside URLs are rejected before scraping starts.
- Verify cancellation stops an active scrape and leaves the UI ready for another run.
- Verify the top 100 table is sorted by post count and tie-broken deterministically.
- Verify copy, select-all, and CSV export preserve Korean text, UID, IP, rank, and count.

## Security And Privacy

- Confirm no local HTTP server is started by the app.
- Confirm no credentials, cookies, or scraped post contents are persisted.
- Confirm network requests are limited to the requested DCInside gallery list pages.
- Confirm generated CSV files contain only the visible ranking fields.

## Performance And Reliability

- Verify retry behavior for first-call, Nth-call, and continuous network failures.
- Verify timeout or cancellation does not leave a scrape marked as running.
- Verify a large page count remains responsive enough to cancel.
- Verify response bodies are closed on success, 4xx, and 5xx responses.

## Compatibility

- Run `scripts/pre-commit.ps1`.
- Run `scripts/pre-release.ps1` on a machine with a C compiler available for `go test -race`.
- Build and smoke-test the Wails executable on each target OS before publishing that OS artifact.
- Confirm WebView2 availability or installer guidance on Windows target machines.

## Rollback Readiness

- Confirm the previous release artifact and tag are still available.
- Confirm the release notes identify the previous stable version.
- Confirm no irreversible data migration exists in this release.
- Confirm the branch can be reverted by one merge revert if the release is pulled.
