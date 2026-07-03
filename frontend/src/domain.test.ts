import { describe, expect, it } from 'vitest';
import {
  escapeCSVField,
  isValidDCGalleryUrl,
  resultSummary,
  safeFilePart,
  timestampForFilename,
} from './domain';

describe('isValidDCGalleryUrl', () => {
  it.each([
    'https://gall.dcinside.com/mini/board/lists/?id=vsoop',
    'https://gall.dcinside.com/mini/vsoop',
    'https://gall.dcinside.com/mgallery/board/lists/?id=test_gallery',
    'https://gall.dcinside.com/mgallery/test_gallery',
    'https://gall.dcinside.com/board/lists/?id=baseball_new11',
    'gall.dcinside.com/mini/vsoop',
  ])('accepts %s', (url) => {
    expect(isValidDCGalleryUrl(url)).toBe(true);
  });

  it.each([
    '',
    'not a url',
    'https://example.com/mini/vsoop',
    'https://gall.dcinside.com/mini/board/lists/',
    'https://gall.dcinside.com/mini/board/not-lists/extra',
    'https://gall.dcinside.com/mgallery/board/lists/',
    'https://gall.dcinside.com/mgallery/board/not-lists/extra',
    'https://gall.dcinside.com/board/lists/',
  ])('rejects %s', (url) => {
    expect(isValidDCGalleryUrl(url)).toBe(false);
  });
});

describe('csv and filename formatting', () => {
  it('escapes csv fields only when required', () => {
    expect(escapeCSVField('plain')).toBe('plain');
    expect(escapeCSVField('with,comma')).toBe('"with,comma"');
    expect(escapeCSVField('with "quote"')).toBe('"with ""quote"""');
    expect(escapeCSVField('with\nnewline')).toBe('"with\nnewline"');
  });

  it('replaces reserved filename characters', () => {
    expect(safeFilePart('a\\b/c:d*e?f"g<h>i|j')).toBe('a_b_c_d_e_f_g_h_i_j');
  });

  it('uses deterministic timestamp formatting', () => {
    expect(timestampForFilename(new Date(2026, 6, 3, 5, 7))).toBe('20260703_0507');
  });
});

describe('resultSummary', () => {
  it('summarizes page-based results', () => {
    expect(
      resultSummary({
        galleryId: 'vsoop',
        pagesScraped: 3,
        totalPosts: 1200,
        uniqueUsers: 34,
      }),
    ).toBe('vsoop · 3페이지 · 게시물 1,200개 · 사용자 34명');
  });

  it('summarizes date-range results with open boundaries', () => {
    expect(
      resultSummary({
        galleryId: 'vsoop',
        pagesScraped: 8,
        startDate: '2026-07-01',
        endDate: null,
        totalPosts: 12,
        uniqueUsers: 5,
      }),
    ).toBe('vsoop · 2026-07-01 ~ 최신 · 게시물 12개 · 사용자 5명');
  });
});
