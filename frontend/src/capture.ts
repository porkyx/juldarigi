import { main } from '../wailsjs/go/models';
import { collectionCountLabel, resultSummary } from './domain';

export const captureTopLimit = 100;
const capturePadding = 28;
const captureWidth = 720;
const captureFont = 'Inter, "Segoe UI", Arial, sans-serif';
const metricRowHeight = 154;
const metricCardGap = 12;
const metricCardPadding = 18;
const metricRankSize = 52;
const metricContentGap = 18;
const metricValueWidth = 126;
const userRowHeight = 108;

export interface CaptureMetricGroup {
  key: string;
  title: string;
  unit: string;
  rows: main.MetricRank[];
}

export function metricPostLabel(entry: Pick<main.MetricRank, 'postNumber' | 'postTitle'>) {
  const number = entry.postNumber ? `${entry.postNumber}번` : '';
  if (number && entry.postTitle) {
    return `${number} · ${entry.postTitle}`;
  }
  return entry.postTitle || (number ? `${number} 글` : '대상 글');
}

export async function renderCapturePNG(
  result: main.ScrapeResult,
  users: main.UserStat[],
  metricGroups: CaptureMetricGroup[],
) {
  await document.fonts?.ready;

  const rows = users.slice(0, captureTopLimit);
  const metricHeight = metricGroups.reduce(
    (height, group) => height + 46 + Math.max(1, group.rows.length) * (metricRowHeight + metricCardGap) + 14,
    0,
  );
  const rankingHeight = Math.max(1, rows.length) * (userRowHeight + 8);
  const height = capturePadding * 2 + 142 + metricHeight + 64 + rankingHeight;
  const scale = Math.min(2, Math.max(1, Math.floor(window.devicePixelRatio || 1)));
  const canvas = document.createElement('canvas');
  canvas.width = captureWidth * scale;
  canvas.height = height * scale;
  canvas.style.width = `${captureWidth}px`;
  canvas.style.height = `${height}px`;

  const ctx = canvas.getContext('2d');
  if (!ctx) {
    throw new Error('캡처 이미지를 생성할 수 없습니다.');
  }
  ctx.scale(scale, scale);
  ctx.fillStyle = '#f8fafc';
  ctx.fillRect(0, 0, captureWidth, height);

  let y = capturePadding;
  drawText(ctx, '갤창랭킹 수집 결과', capturePadding, y + 40, 42, 800, '#0f172a');
  y += 76;
  drawText(ctx, resultSummary(result), capturePadding, y, 22, 600, '#475569', 'left', captureWidth - capturePadding * 2);
  y += 46;

  drawText(ctx, '사용자별 최고 글 Top 3', capturePadding, y, 32, 800, '#0f172a');
  y += 44;
  for (const group of metricGroups) {
    y = drawCaptureMetricGroup(ctx, group, y);
  }

  y += 24;
  drawText(ctx, `${collectionCountLabel(result.collectionMode)} 랭킹 1 ~ ${rows.length}위`, capturePadding, y, 32, 800, '#0f172a');
  y += 44;
  drawCaptureRankingCards(ctx, rows, y, collectionCountLabel(result.collectionMode));

  return canvas.toDataURL('image/png');
}

function drawCaptureMetricGroup(ctx: CanvasRenderingContext2D, group: CaptureMetricGroup, y: number) {
  const x = capturePadding;
  const width = captureWidth - capturePadding * 2;
  drawText(ctx, group.title, x, y, 26, 800, '#1e293b');
  y += 38;

  const rows = group.rows.length ? group.rows : [];
  if (!rows.length) {
    drawRoundedRect(ctx, x, y, width, 88, 8, '#ffffff', '#d7dde8');
    drawText(ctx, '데이터 없음', x + 24, y + 54, 24, 700, '#64748b');
    return y + 110;
  }

  for (const entry of rows) {
    const contentX = x + metricCardPadding + metricRankSize + metricContentGap;
    const valueX = x + width - metricCardPadding;
    const contentWidth = width - (contentX - x) - metricValueWidth - metricCardPadding;
    const fullContentWidth = width - (contentX - x) - metricCardPadding;
    const topBaseline = y + 38;
    const titleBaseline = y + 72;
    const urlLabelBaseline = y + 107;
    const urlBaseline = y + 134;

    drawRoundedRect(ctx, x, y, width, metricRowHeight, 8, '#ffffff', '#d7dde8');
    drawRoundedRect(ctx, x + metricCardPadding, y + metricCardPadding, metricRankSize, metricRankSize, 8, '#e8f5f3', '#b7d8d2');
    drawText(ctx, `${entry.rank}`, x + metricCardPadding + metricRankSize / 2, y + 54, 28, 800, '#0f766e', 'center');
    drawText(ctx, entry.nickname || '-', contentX, topBaseline, 25, 800, '#0f172a', 'left', contentWidth);
    drawText(ctx, `${entry.value.toLocaleString()}${group.unit}`, valueX, topBaseline, 25, 800, '#0f766e', 'right', metricValueWidth);
    drawText(ctx, metricPostLabel(entry), contentX, titleBaseline, 21, 600, '#334155', 'left', fullContentWidth);
    drawText(ctx, '원본 URL', contentX, urlLabelBaseline, 16, 800, '#64748b');
    drawText(ctx, entry.postUrl || '-', contentX, urlBaseline, 18, 600, '#2563eb', 'left', fullContentWidth);
    y += metricRowHeight + metricCardGap;
  }
  return y + 12;
}

function drawCaptureRankingCards(ctx: CanvasRenderingContext2D, users: main.UserStat[], y: number, countLabel: string) {
  const x = capturePadding;
  const width = captureWidth - capturePadding * 2;

  if (!users.length) {
    drawRoundedRect(ctx, x, y, width, 88, 8, '#ffffff', '#d7dde8');
    drawText(ctx, '데이터 없음', x + 24, y + 54, 24, 700, '#64748b');
    return;
  }

  users.forEach((user, index) => {
    const fill = index % 2 === 0 ? '#ffffff' : '#f8fafc';
    drawRoundedRect(ctx, x, y, width, userRowHeight, 8, fill, '#d7dde8');
    drawText(ctx, `#${index + 1}`, x + 24, y + 42, 30, 800, '#334155');
    drawText(ctx, user.nickname || '-', x + 116, y + 34, 26, 800, '#0f172a', 'left', width - 304);
    drawText(ctx, `${user.count.toLocaleString()}개`, x + width - 24, y + 40, 30, 800, '#0f766e', 'right');
    drawText(ctx, countLabel, x + width - 24, y + 68, 18, 700, '#64748b', 'right');
    drawText(ctx, `식별코드 ${user.uid || '-'}`, x + 116, y + 70, 20, 600, '#334155', 'left', width - 148);
    drawText(ctx, `IP ${user.ip || '-'}`, x + 116, y + 96, 20, 600, '#475569', 'left', width - 148);
    y += userRowHeight + 8;
  });
}

function drawText(
  ctx: CanvasRenderingContext2D,
  text: string,
  x: number,
  y: number,
  size: number,
  weight: number,
  color: string,
  align: CanvasTextAlign = 'left',
  maxWidth?: number,
) {
  ctx.font = `${weight} ${size}px ${captureFont}`;
  ctx.fillStyle = color;
  ctx.textAlign = align;
  ctx.textBaseline = 'alphabetic';
  const output = maxWidth ? truncateCanvasText(ctx, text, maxWidth) : text;
  ctx.fillText(output, x, y);
}

export function truncateCanvasText(ctx: Pick<CanvasRenderingContext2D, 'measureText'>, text: string, maxWidth: number) {
  if (ctx.measureText(text).width <= maxWidth) {
    return text;
  }
  let output = text;
  while (output.length > 1 && ctx.measureText(`${output}…`).width > maxWidth) {
    output = output.slice(0, -1);
  }
  return `${output}…`;
}

function drawRoundedRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
  fill: string,
  stroke: string,
) {
  ctx.beginPath();
  ctx.roundRect(x, y, width, height, radius);
  ctx.fillStyle = fill;
  ctx.fill();
  ctx.strokeStyle = stroke;
  ctx.stroke();
}
