// Human-readable formatting that mirrors Gdu's own size formatting so the
// numbers shown in the browser match the terminal UI.

const BINARY_PREFIXES = ['', 'Ki', 'Mi', 'Gi', 'Ti', 'Pi', 'Ei'];
const SI_PREFIXES = ['', 'k', 'M', 'G', 'T', 'P', 'E'];

export function formatSize(bytes: number, useSIPrefix = false): string {
  const base = useSIPrefix ? 1000 : 1024;
  const prefixes = useSIPrefix ? SI_PREFIXES : BINARY_PREFIXES;

  let value = Math.abs(bytes);
  let i = 0;
  while (value >= base && i < prefixes.length - 1) {
    value /= base;
    i++;
  }

  const sign = bytes < 0 ? '-' : '';
  const digits = value < 10 && i > 0 ? 1 : 0;
  const unit = `${prefixes[i]}B`;
  return `${sign}${value.toFixed(digits)} ${unit}`;
}

export function formatCount(n: number): string {
  return n.toLocaleString('en-US');
}

export function formatMtime(unixSeconds: number): string {
  if (!unixSeconds) {
    return '';
  }
  const d = new Date(unixSeconds * 1000);
  return d.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

// percent returns value/total as a bounded 0..100 number.
export function percent(value: number, total: number): number {
  if (total <= 0) {
    return 0;
  }
  const p = (value / total) * 100;
  if (p < 0) return 0;
  if (p > 100) return 100;
  return p;
}
