import { useEffect, useMemo, useRef, useState } from 'react';
import {
  CalendarDays,
  Clipboard,
  Download,
  Eye,
  FileText,
  Loader2,
  MessageSquare,
  MousePointer,
  Play,
  Square,
  ThumbsUp,
} from 'lucide-react';
import { CancelScrape, ScrapeDCGallery } from '../wailsjs/go/main/App';
import { main } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';
import {
  escapeCSVField,
  isValidDCGalleryUrl,
  resultSummary,
  safeFilePart,
  timestampForFilename,
} from './domain';

type ScrapeMode = 'pages' | 'dates';

interface ProgressPayload {
  currentPage: number;
  totalPages?: number;
  totalPosts: number;
  uniqueUsers: number;
  message: string;
}

interface MessagePayload {
  message: string;
}

const topLimit = 100;

function metricPostLabel(entry: main.MetricRank) {
  const number = entry.postNumber ? `${entry.postNumber}번` : '';
  if (number && entry.postTitle) {
    return `${number} · ${entry.postTitle}`;
  }
  return entry.postTitle || (number ? `${number} 글` : '대상 글');
}

function App() {
  const [url, setUrl] = useState('');
  const [mode, setMode] = useState<ScrapeMode>('pages');
  const [pages, setPages] = useState(1);
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [progress, setProgress] = useState<ProgressPayload | null>(null);
  const [result, setResult] = useState<main.ScrapeResult | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const tableRef = useRef<HTMLTableElement | null>(null);

  const isValidUrl = useMemo(() => isValidDCGalleryUrl(url), [url]);
  const visibleUsers = result?.userStats.slice(0, topLimit) ?? [];
  const metricGroups = useMemo(
    () => [
      {
        key: 'views',
        title: '조회수 Top 3',
        unit: '회',
        rows: result?.topMetrics?.views ?? [],
        Icon: Eye,
      },
      {
        key: 'recommendations',
        title: '추천 Top 3',
        unit: '개',
        rows: result?.topMetrics?.recommendations ?? [],
        Icon: ThumbsUp,
      },
      {
        key: 'comments',
        title: '댓글 Top 3',
        unit: '개',
        rows: result?.topMetrics?.comments ?? [],
        Icon: MessageSquare,
      },
    ],
    [result],
  );
  const hasMetricStats = metricGroups.some((group) => group.rows.length > 0);

  useEffect(() => {
    const unsubscribeProgress = EventsOn('scrape:progress', (payload: ProgressPayload) => {
      setProgress(payload);
    });
    const unsubscribeInfo = EventsOn('scrape:info', (payload: MessagePayload) => {
      setProgress({
        currentPage: 0,
        totalPages: 0,
        totalPosts: 0,
        uniqueUsers: 0,
        message: payload.message,
      });
    });
    const unsubscribeWarning = EventsOn('scrape:warning', (payload: MessagePayload) => {
      setNotice(payload.message);
    });

    return () => {
      unsubscribeProgress();
      unsubscribeInfo();
      unsubscribeWarning();
    };
  }, []);

  async function startScraping() {
    if (!isValidUrl || isLoading) {
      return;
    }

    setIsLoading(true);
    setProgress(null);
    setResult(null);
    setError('');
    setNotice('');

    const request: main.ScrapeRequest = {
      url,
      pages: mode === 'pages' ? Math.max(1, pages) : 0,
      startDate: mode === 'dates' ? startDate : '',
      endDate: mode === 'dates' ? endDate : '',
    };

    try {
      const scrapeResult = await ScrapeDCGallery(request);
      setResult(scrapeResult);
      setNotice('완료되었습니다.');
    } catch (scrapeError) {
      setError(scrapeError instanceof Error ? scrapeError.message : String(scrapeError));
    } finally {
      setIsLoading(false);
      setProgress(null);
    }
  }

  async function cancelScraping() {
    await CancelScrape();
    setIsLoading(false);
  }

  async function copyTableData() {
    if (!visibleUsers.length) {
      return;
    }

    const rows = [
      ['순위', '닉네임', '식별코드', 'IP', '게시물 수'].join('\t'),
      ...visibleUsers.map((user, index) =>
        [index + 1, user.nickname, user.uid, user.ip, `${user.count}개`].join('\t'),
      ),
    ];
    const text = rows.join('\n');

    try {
      await navigator.clipboard.writeText(text);
      setNotice('테이블 데이터가 복사되었습니다.');
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.style.position = 'fixed';
      textarea.style.left = '-9999px';
      document.body.appendChild(textarea);
      textarea.focus();
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      setNotice('테이블 데이터가 복사되었습니다.');
    }
  }

  function selectTableContent() {
    if (!tableRef.current) {
      return;
    }

    const range = document.createRange();
    range.selectNodeContents(tableRef.current);
    const selection = window.getSelection();
    selection?.removeAllRanges();
    selection?.addRange(range);
    setNotice('테이블이 선택되었습니다.');
  }

  function exportToCSV() {
    if (!result || !visibleUsers.length) {
      return;
    }

    const csvRows = [
      ['게시글 수 랭킹'].join(','),
      ['순위', '닉네임', '식별코드', 'IP', '게시물수'].join(','),
      ...visibleUsers.map((user, index) =>
        [index + 1, escapeCSVField(user.nickname), escapeCSVField(user.uid), escapeCSVField(user.ip), user.count].join(','),
      ),
    ];
    for (const group of metricGroups) {
      if (!group.rows.length) {
        continue;
      }
      csvRows.push('');
      csvRows.push(group.title);
      csvRows.push(['순위', '닉네임', '식별코드', 'IP', `값(${group.unit})`, '글번호', '제목', 'URL'].join(','));
      csvRows.push(
        ...group.rows.map((entry) =>
          [
            entry.rank,
            escapeCSVField(entry.nickname),
            escapeCSVField(entry.uid),
            escapeCSVField(entry.ip),
            entry.value,
            escapeCSVField(entry.postNumber),
            escapeCSVField(entry.postTitle),
            escapeCSVField(entry.postUrl),
          ].join(','),
        ),
      );
    }

    const blob = new Blob([`\uFEFF${csvRows.join('\n')}`], {
      type: 'text/csv;charset=utf-8;',
    });
    const downloadUrl = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = downloadUrl;
    link.download = `갤창랭킹_${safeFilePart(result.galleryId)}_${timestampForFilename()}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(downloadUrl);
    setNotice('CSV 파일을 생성했습니다.');
  }

  return (
    <main className="app-shell">
      <header className="app-header">
        <div>
          <h1>갤창랭킹 수집기</h1>
          <p>{result ? `${result.galleryId} · ${result.totalPosts.toLocaleString()}개 게시물` : '줄다리기'}</p>
        </div>
        {isLoading ? (
          <button className="icon-button danger" type="button" onClick={cancelScraping} title="중지">
            <Square size={18} />
            <span>중지</span>
          </button>
        ) : null}
      </header>

      <section className="control-band">
        <div className="url-row">
          <label htmlFor="gallery-url">Gallery URL</label>
          <input
            id="gallery-url"
            className="url-input"
            value={url}
            onChange={(event) => setUrl(event.target.value)}
            placeholder="https://gall.dcinside.com/mini/vsoop"
            spellCheck={false}
          />
        </div>

        <div className="mode-row" role="tablist" aria-label="스크래핑 모드">
          <button
            className={mode === 'pages' ? 'segment active' : 'segment'}
            type="button"
            onClick={() => setMode('pages')}
          >
            <FileText size={16} />
            페이지
          </button>
          <button
            className={mode === 'dates' ? 'segment active' : 'segment'}
            type="button"
            onClick={() => setMode('dates')}
          >
            <CalendarDays size={16} />
            날짜
          </button>
        </div>

        {mode === 'pages' ? (
          <div className="field-row">
            <label htmlFor="pages">페이지 수</label>
            <input
              id="pages"
              className="compact-input"
              type="number"
              min={1}
              value={pages}
              onChange={(event) => setPages(Number(event.target.value) || 1)}
            />
          </div>
        ) : (
          <div className="date-grid">
            <div className="field-row">
              <label htmlFor="start-date">시작 날짜</label>
              <input id="start-date" type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} />
            </div>
            <div className="field-row">
              <label htmlFor="end-date">종료 날짜</label>
              <input id="end-date" type="date" value={endDate} onChange={(event) => setEndDate(event.target.value)} />
            </div>
          </div>
        )}

        <button
          className="primary-action"
          type="button"
          disabled={!isValidUrl || isLoading}
          onClick={startScraping}
        >
          {isLoading ? <Loader2 className="spin" size={18} /> : <Play size={18} />}
          {isLoading ? '스크래핑 중' : '스크래핑 시작'}
        </button>
      </section>

      {progress ? (
        <section className="progress-band" aria-live="polite">
          <div>
            <strong>{progress.message}</strong>
            <span>
              {progress.totalPages
                ? `${progress.currentPage} / ${progress.totalPages} 페이지`
                : `${progress.currentPage} 페이지`}
            </span>
          </div>
          {progress.totalPages ? (
            <progress max={progress.totalPages} value={progress.currentPage} />
          ) : null}
          <dl>
            <div>
              <dt>게시물</dt>
              <dd>{progress.totalPosts.toLocaleString()}</dd>
            </div>
            <div>
              <dt>사용자</dt>
              <dd>{progress.uniqueUsers.toLocaleString()}</dd>
            </div>
          </dl>
        </section>
      ) : null}

      {error ? <section className="message error">{error}</section> : null}
      {notice && !error ? <section className="message">{notice}</section> : null}

      {result ? (
        <section className="results-band">
          <div className="results-heading">
            <div>
              <h2>1 ~ {Math.min(topLimit, result.userStats.length)}위 게시글 수 랭킹</h2>
              <p>{resultSummary(result)}</p>
            </div>
            <div className="table-actions">
              <button type="button" onClick={copyTableData} title="복사">
                <Clipboard size={16} />
                복사
              </button>
              <button type="button" onClick={selectTableContent} title="전체선택">
                <MousePointer size={16} />
                전체선택
              </button>
              <button type="button" onClick={exportToCSV} title="CSV 내보내기">
                <Download size={16} />
                CSV
              </button>
            </div>
          </div>

          {hasMetricStats ? (
            <div className="metric-summary" aria-label="사용자별 최고 글 통계">
              {metricGroups.map((group) => {
                const Icon = group.Icon;
                return (
                  <div className="metric-group" key={group.key}>
                    <h3>
                      <Icon size={15} />
                      {group.title}
                    </h3>
                    <ol>
                      {group.rows.map((entry) => (
                        <li key={`${group.key}-${entry.uid}`}>
                          <span className="metric-rank">{entry.rank}</span>
                          <div>
                            <strong>{entry.nickname}</strong>
                            {entry.postUrl ? (
                              <a href={entry.postUrl} target="_blank" rel="noreferrer">
                                {metricPostLabel(entry)}
                              </a>
                            ) : (
                              <span>{metricPostLabel(entry)}</span>
                            )}
                          </div>
                          <em>
                            {entry.value.toLocaleString()}
                            {group.unit}
                          </em>
                        </li>
                      ))}
                    </ol>
                  </div>
                );
              })}
            </div>
          ) : null}

          <div className="table-wrap">
            <table ref={tableRef}>
              <thead>
                <tr>
                  <th>순위</th>
                  <th>닉네임</th>
                  <th>식별코드</th>
                  <th>IP</th>
                  <th>게시물 수</th>
                </tr>
              </thead>
              <tbody>
                {visibleUsers.map((user, index) => (
                  <tr key={user.uid}>
                    <td>{index + 1}</td>
                    <td>{user.nickname}</td>
                    <td className="mono">{user.uid}</td>
                    <td className="mono">{user.ip}</td>
                    <td>{user.count.toLocaleString()}개</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}
    </main>
  );
}

export default App;
