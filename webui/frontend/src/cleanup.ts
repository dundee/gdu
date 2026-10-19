// Projections color the treemap by something other than location, to draw
// attention to space that is likely safe to free. Area always stays size.
import type { Category, Node } from './types';
import { formatSize } from './format';
import { PALETTE } from './colors';

export type Projection = 'location' | 'cleanup' | 'age' | 'categories';

export const PROJECTIONS: Array<{ id: Projection; label: string }> = [
  { id: 'location', label: 'Location' },
  { id: 'cleanup', label: 'Cleanup' },
  { id: 'age', label: 'Age' },
  { id: 'categories', label: 'By category' },
];

// Path prefix of the synthetic category nodes of a pivoted tree
// (pivotPathPrefix in webui/cleanup.go). They are not real items.
export const PIVOT_PATH_PREFIX = 'category:';

export function isPivotNode(node: Node): boolean {
  return node.path.startsWith(PIVOT_PATH_PREFIX);
}

// Everything that is not a candidate recedes into this grey, so candidates
// stand out by contrast rather than by a rainbow of hues.
export const NEUTRAL_COLOR = '#353b47';
export const SAFE_COLOR = '#f08c00';
export const REVIEW_COLOR = '#c9a25e';

export const CATEGORY_COLORS: Record<Category, string> = {
  'build-artifact': PALETTE[1],
  'package-cache': PALETTE[5],
  'app-cache': PALETTE[6],
  toolchain: PALETTE[9],
  'ml-model': PALETTE[4],
  installer: PALETTE[0],
  'git-ignored': PALETTE[10],
};

function amountOf(node: Node, apparent: boolean): number {
  return apparent ? node.size : node.usage;
}

// candidateBytes is the space taken by candidates within node.
export function candidateBytes(node: Node, apparent: boolean): number {
  let sum = 0;
  for (const amount of Object.values(node.categoryTotals ?? {})) {
    if (amount) {
      sum += apparent ? amount.size : amount.usage;
    }
  }
  return sum;
}

// candidateShare is the fraction (0..1) of node taken by candidates.
export function candidateShare(node: Node, apparent: boolean): number {
  const total = amountOf(node, apparent);
  if (total <= 0) {
    return 0;
  }
  return Math.min(candidateBytes(node, apparent) / total, 1);
}

function mix(from: string, to: string, t: number): string {
  const channel = (hex: string, i: number) => parseInt(hex.slice(1 + i * 2, 3 + i * 2), 16);
  const out = [0, 1, 2].map((i) =>
    Math.round(channel(from, i) + (channel(to, i) - channel(from, i)) * t)
      .toString(16)
      .padStart(2, '0'),
  );
  return `#${out.join('')}`;
}

// cleanupColor colors a candidate by confidence; a directory holding
// candidates is tinted by their share, so its frame shows how much of it is
// reclaimable even where its children are too small to draw.
export function cleanupColor(node: Node, apparent: boolean): string {
  if (node.confidence === 'safe') {
    return SAFE_COLOR;
  }
  if (node.confidence === 'review') {
    return REVIEW_COLOR;
  }
  const share = candidateShare(node, apparent);
  return share > 0 ? mix(NEUTRAL_COLOR, SAFE_COLOR, 0.25 + share * 0.5) : NEUTRAL_COLOR;
}

const DAY = 24 * 60 * 60;

export const AGE_BUCKETS: Array<{ maxDays: number; label: string; color: string }> = [
  { maxDays: 30, label: '< 1 month', color: NEUTRAL_COLOR },
  { maxDays: 182, label: '< 6 months', color: '#5b5440' },
  { maxDays: 365, label: '< 1 year', color: '#9a7330' },
  { maxDays: Infinity, label: 'older', color: SAFE_COLOR },
];

// ageColor colors by the latest change within the item: the longer nothing
// changed, the stronger the color.
export function ageColor(mtime: number, now: number): string {
  if (mtime <= 0) {
    return NEUTRAL_COLOR;
  }
  const days = (now - mtime) / DAY;
  return (AGE_BUCKETS.find((b) => days < b.maxDays) ?? AGE_BUCKETS[AGE_BUCKETS.length - 1]).color;
}

// ageText describes how long ago the item last changed, coarsely.
export function ageText(mtime: number, now: number): string {
  const days = Math.floor((now - mtime) / DAY);
  if (mtime <= 0 || days < 1) {
    return 'changed today';
  }
  if (days < 60) {
    return `unchanged for ${days} day${days === 1 ? '' : 's'}`;
  }
  if (days < 730) {
    return `unchanged for ${Math.floor(days / 30)} months`;
  }
  return `unchanged for ${Math.floor(days / 365)} years`;
}

// candidateSummary is the tooltip line for an item holding candidates, e.g.
// "14 GB build artifacts · unchanged for 4 months".
export function candidateSummary(
  node: Node,
  apparent: boolean,
  useSIPrefix: boolean,
  labels: Map<Category, string>,
  now: number,
): string | null {
  const entries = Object.entries(node.categoryTotals ?? {})
    .map(([category, amount]) => ({
      category: category as Category,
      bytes: amount ? (apparent ? amount.size : amount.usage) : 0,
    }))
    .filter((e) => e.bytes > 0)
    .sort((a, b) => b.bytes - a.bytes);
  if (entries.length === 0) {
    return null;
  }
  const parts = entries
    .slice(0, 2)
    .map((e) => `${formatSize(e.bytes, useSIPrefix)} ${(labels.get(e.category) ?? e.category).toLowerCase()}`);
  if (entries.length > 2) {
    parts.push(`+${entries.length - 2} more`);
  }
  return `${parts.join(', ')} · ${ageText(node.mtime, now)}`;
}

// containsCategory reports whether a legend filter keeps node highlighted.
export function containsCategory(node: Node, category: Category): boolean {
  const amount = node.categoryTotals?.[category];
  return node.category === category || (amount !== undefined && (amount.usage > 0 || amount.size > 0));
}
