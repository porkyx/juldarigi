export interface ResultSummaryInput {
  galleryId: string;
  pagesScraped: number;
  startDate?: string | null;
  endDate?: string | null;
  collectionMode?: string;
  totalPosts: number;
  totalComments?: number;
  uniqueUsers: number;
}

export type CollectionMode = 'posts' | 'posts_comments' | 'comments';

export function collectionModeLabel(mode?: string) {
  switch (mode) {
    case 'posts_comments':
      return '게시글 + 댓글 수집';
    case 'comments':
      return '댓글 수집';
    default:
      return '게시글 수집';
  }
}

export function collectionCountLabel(mode?: string) {
  switch (mode) {
    case 'posts_comments':
      return '활동 수';
    case 'comments':
      return '댓글 수';
    default:
      return '게시글 수';
  }
}

export function isValidDCGalleryUrl(value: string) {
  try {
    const target = value.startsWith('http://') || value.startsWith('https://') ? value : `https://${value}`;
    const parsed = new URL(target);
    if (parsed.hostname !== 'gall.dcinside.com') {
      return false;
    }

    const path = parsed.pathname.replace(/^\/+|\/+$/g, '').split('/').filter(Boolean);
    if (path[0] === 'mini' && path[1] === 'board' && path[2] === 'lists') {
      return Boolean(parsed.searchParams.get('id'));
    }
    if (path[0] === 'mini' && path[1] === 'board') {
      return false;
    }
    if (path[0] === 'mgallery' && path[1] === 'board' && path[2] === 'lists') {
      return Boolean(parsed.searchParams.get('id'));
    }
    if (path[0] === 'mgallery' && path[1] === 'board') {
      return false;
    }
    if (path[0] === 'board' && path[1] === 'lists') {
      return Boolean(parsed.searchParams.get('id'));
    }
    return path[0] === 'mini' || path[0] === 'mgallery' ? Boolean(path[1]) : path.length === 1;
  } catch {
    return false;
  }
}

export function resultSummary(result: ResultSummaryInput) {
  const period =
    result.startDate || result.endDate
      ? `${result.startDate ?? '처음'} ~ ${result.endDate ?? '최신'}`
      : `${result.pagesScraped.toLocaleString()}페이지`;
  const commentPart =
    result.collectionMode === 'comments' || result.collectionMode === 'posts_comments'
      ? ` · 댓글 ${(result.totalComments ?? 0).toLocaleString()}개`
      : '';
  return `${result.galleryId} · ${period} · ${collectionModeLabel(result.collectionMode)} · 게시물 ${result.totalPosts.toLocaleString()}개${commentPart} · 사용자 ${result.uniqueUsers.toLocaleString()}명`;
}

export function escapeCSVField(field: string) {
  if (field.includes(',') || field.includes('"') || field.includes('\n')) {
    return `"${field.replace(/"/g, '""')}"`;
  }
  return field;
}

export function safeFilePart(value: string) {
  return value.replace(/[\\/:*?"<>|]/g, '_');
}

export function timestampForFilename(now = new Date()) {
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  const hour = String(now.getHours()).padStart(2, '0');
  const minute = String(now.getMinutes()).padStart(2, '0');
  return `${year}${month}${day}_${hour}${minute}`;
}
