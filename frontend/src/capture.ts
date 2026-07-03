import { main } from '../wailsjs/go/models';
import { resultSummary } from './domain';

export const captureTopLimit = 100;
const capturePadding = 40;
const captureWidth = 1440;
const captureFont = 'Inter, "Segoe UI", Arial, sans-serif';

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
  const metricHeight = metricGroups.reduce((height, group) => height + 38 + Math.max(1, group.rows.length) * 82 + 18, 0);
  const tableHeight = 48 + rows.length * 34;
  const height = capturePadding * 2 + 112 + metricHeight + 52 + tableHeight;
  const scale = Math.max(1, Math.floor(window.devicePixelRatio || 1));
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
  drawText(ctx, '갤창랭킹 수집 결과', capturePadding, y, 30, 800, '#0f172a');
  y += 38;
  drawText(ctx, resultSummary(result), capturePadding, y, 16, 500, '#475569');
  y += 44;

  drawText(ctx, '사용자별 최고 글 Top 3', capturePadding, y, 22, 800, '#0f172a');
  y += 34;
  for (const group of metricGroups) {
    y = drawCaptureMetricGroup(ctx, group, y);
  }

  y += 18;
  drawText(ctx, `게시글 수 랭킹 1 ~ ${rows.length}위`, capturePadding, y, 22, 800, '#0f172a');
  y += 30;
  drawCaptureTable(ctx, rows, y);

  return canvas.toDataURL('image/png');
}

function drawCaptureMetricGroup(ctx: CanvasRenderingContext2D, group: CaptureMetricGroup, y: number) {
  const x = capturePadding;
  const width = captureWidth - capturePadding * 2;
  drawText(ctx, group.title, x, y, 18, 800, '#1e293b');
  y += 28;

  const rows = group.rows.length ? group.rows : [];
  if (!rows.length) {
    drawText(ctx, '데이터 없음', x, y + 22, 14, 500, '#64748b');
    return y + 82;
  }

  for (const entry of rows) {
    drawRoundedRect(ctx, x, y, width, 72, 6, '#ffffff', '#d7dde8');
    drawText(ctx, `${entry.rank}`, x + 18, y + 28, 18, 800, '#475569', 'center');
    drawText(ctx, entry.nickname, x + 52, y + 22, 15, 800, '#0f172a');
    drawText(ctx, metricPostLabel(entry), x + 52, y + 43, 13, 500, '#475569', 'left', 760);
    drawText(ctx, entry.postUrl || '-', x + 52, y + 62, 12, 500, '#2563eb', 'left', 960);
    drawText(ctx, `${entry.value.toLocaleString()}${group.unit}`, x + width - 22, y + 40, 16, 800, '#0f766e', 'right');
    y += 82;
  }
  return y + 8;
}

function drawCaptureTable(ctx: CanvasRenderingContext2D, users: main.UserStat[], y: number) {
  const x = capturePadding;
  const width = captureWidth - capturePadding * 2;
  const columns = [
    { title: '순위', x: x + 20, width: 80 },
    { title: '닉네임', x: x + 110, width: 300 },
    { title: '식별코드', x: x + 420, width: 430 },
    { title: 'IP', x: x + 860, width: 180 },
    { title: '게시물 수', x: x + width - 160, width: 140 },
  ];

  drawRoundedRect(ctx, x, y, width, 40, 6, '#e8edf5', '#d7dde8');
  for (const column of columns) {
    drawText(ctx, column.title, column.x, y + 25, 13, 800, '#334155');
  }
  y += 40;

  users.forEach((user, index) => {
    const fill = index % 2 === 0 ? '#ffffff' : '#f8fafc';
    ctx.fillStyle = fill;
    ctx.fillRect(x, y, width, 34);
    ctx.strokeStyle = '#e2e8f0';
    ctx.beginPath();
    ctx.moveTo(x, y + 34);
    ctx.lineTo(x + width, y + 34);
    ctx.stroke();

    drawText(ctx, String(index + 1), columns[0].x, y + 22, 13, 700, '#334155', 'left', columns[0].width);
    drawText(ctx, user.nickname, columns[1].x, y + 22, 13, 500, '#0f172a', 'left', columns[1].width);
    drawText(ctx, user.uid, columns[2].x, y + 22, 13, 500, '#334155', 'left', columns[2].width);
    drawText(ctx, user.ip || '-', columns[3].x, y + 22, 13, 500, '#334155', 'left', columns[3].width);
    drawText(ctx, `${user.count.toLocaleString()}개`, columns[4].x, y + 22, 13, 700, '#0f766e', 'left', columns[4].width);
    y += 34;
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
