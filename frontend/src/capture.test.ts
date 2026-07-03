import { describe, expect, it, vi } from 'vitest';
import { metricPostLabel, renderCapturePNG, truncateCanvasText } from './capture';
import { main } from '../wailsjs/go/models';

describe('metricPostLabel', () => {
  it.each([
    [{ postNumber: '123', postTitle: '제목' }, '123번 · 제목'],
    [{ postNumber: '', postTitle: '제목만' }, '제목만'],
    [{ postNumber: '456', postTitle: '' }, '456번 글'],
    [{ postNumber: '', postTitle: '' }, '대상 글'],
  ])('formats %o as %s', (entry, expected) => {
    expect(metricPostLabel(entry)).toBe(expected);
  });
});

describe('truncateCanvasText', () => {
  const ctx = {
    measureText: (text: string) => ({ width: text.length * 10 }) as TextMetrics,
  };

  it('keeps text that already fits', () => {
    expect(truncateCanvasText(ctx, 'short', 60)).toBe('short');
  });

  it('adds an ellipsis when text exceeds the width', () => {
    expect(truncateCanvasText(ctx, 'abcdef', 35)).toBe('ab…');
  });
});

describe('renderCapturePNG', () => {
  it('renders metric groups and the top 100 users to a PNG data URL', async () => {
    const previousDocument = globalThis.document;
    const previousWindow = globalThis.window;
    const harness = createCanvasHarness();
    vi.stubGlobal('document', {
      fonts: { ready: Promise.resolve() },
      createElement: (tagName: string) => {
        if (tagName !== 'canvas') {
          throw new Error(`unexpected tag: ${tagName}`);
        }
        return harness.canvas;
      },
    });
    vi.stubGlobal('window', { devicePixelRatio: 2 });

    const users = Array.from(
      { length: 101 },
      (_, index) =>
        new main.UserStat({
          uid: `uid-${index + 1}`,
          nickname: `Nick ${index + 1}`,
          ip: index % 2 === 0 ? '' : '127.0.0.1',
          count: index + 1,
        }),
    );
    const result = new main.ScrapeResult({
      galleryId: 'spv',
      pagesScraped: 1,
      totalPosts: users.length,
      uniqueUsers: users.length,
      userStats: users,
      topMetrics: { views: [], recommendations: [], comments: [] },
    });

    try {
      const dataUrl = await renderCapturePNG(result, users, [
        {
          key: 'views',
          title: '조회수 Top 3',
          unit: '회',
          rows: [
            new main.MetricRank({
              rank: 1,
              uid: 'uid-1',
              nickname: 'Nick 1',
              ip: '',
              value: 1234,
              postNumber: '649328',
              postTitle: '캡처 대상 글',
              postUrl: 'https://gall.dcinside.com/mini/board/view/?id=spv&no=649328',
            }),
          ],
        },
        {
          key: 'recommendations',
          title: '추천 Top 3',
          unit: '개',
          rows: [],
        },
      ]);

      const renderedTexts = harness.fillTextCalls.map((call) => call.text);
      expect(dataUrl).toBe('data:image/png;base64,test');
      expect(harness.canvas.width).toBe(2880);
      expect(harness.canvas.height).toBeGreaterThan(0);
      expect(harness.context.scale).toHaveBeenCalledWith(2, 2);
      expect(renderedTexts).toContain('갤창랭킹 수집 결과');
      expect(renderedTexts).toContain('게시글 수 랭킹 1 ~ 100위');
      expect(renderedTexts).toContain('649328번 · 캡처 대상 글');
      expect(renderedTexts).toContain('https://gall.dcinside.com/mini/board/view/?id=spv&no=649328');
      expect(renderedTexts).toContain('데이터 없음');
      expect(renderedTexts).toContain('Nick 100');
      expect(renderedTexts).not.toContain('Nick 101');
    } finally {
      vi.stubGlobal('document', previousDocument);
      vi.stubGlobal('window', previousWindow);
    }
  });

  it('throws when a canvas context cannot be created', async () => {
    const previousDocument = globalThis.document;
    const previousWindow = globalThis.window;
    vi.stubGlobal('document', {
      fonts: { ready: Promise.resolve() },
      createElement: (tagName: string) => {
        if (tagName !== 'canvas') {
          throw new Error(`unexpected tag: ${tagName}`);
        }
        return {
          style: {},
          getContext: () => null,
        };
      },
    });
    vi.stubGlobal('window', { devicePixelRatio: 1 });

    try {
      await expect(
        renderCapturePNG(
          new main.ScrapeResult({
            galleryId: 'spv',
            pagesScraped: 1,
            totalPosts: 1,
            uniqueUsers: 1,
            userStats: [],
            topMetrics: { views: [], recommendations: [], comments: [] },
          }),
          [],
          [],
        ),
      ).rejects.toThrow('캡처 이미지를 생성할 수 없습니다.');
    } finally {
      vi.stubGlobal('document', previousDocument);
      vi.stubGlobal('window', previousWindow);
    }
  });
});

function createCanvasHarness() {
  const fillTextCalls: Array<{ text: string; x: number; y: number }> = [];
  const context = {
    font: '',
    fillStyle: '',
    strokeStyle: '',
    textAlign: 'left' as CanvasTextAlign,
    textBaseline: 'alphabetic' as CanvasTextBaseline,
    beginPath: vi.fn(),
    fill: vi.fn(),
    fillRect: vi.fn(),
    fillText: vi.fn((text: string, x: number, y: number) => {
      fillTextCalls.push({ text, x, y });
    }),
    lineTo: vi.fn(),
    measureText: (text: string) => ({ width: text.length * 4 }) as TextMetrics,
    moveTo: vi.fn(),
    roundRect: vi.fn(),
    scale: vi.fn(),
    stroke: vi.fn(),
  };
  const canvas = {
    height: 0,
    style: {} as Record<string, string>,
    width: 0,
    getContext: vi.fn(() => context),
    toDataURL: vi.fn(() => 'data:image/png;base64,test'),
  };

  return { canvas, context, fillTextCalls };
}
