import { describe, expect, it } from 'vitest';
import { formatSize, formatCount, percent } from './format';

describe('formatSize', () => {
  it('formats bytes with binary prefixes by default', () => {
    expect(formatSize(0)).toBe('0 B');
    expect(formatSize(512)).toBe('512 B');
    expect(formatSize(1024)).toBe('1.0 KiB');
    expect(formatSize(1536)).toBe('1.5 KiB');
    expect(formatSize(1048576)).toBe('1.0 MiB');
  });

  it('formats bytes with SI prefixes when requested', () => {
    expect(formatSize(1000, true)).toBe('1.0 kB');
    expect(formatSize(1000000, true)).toBe('1.0 MB');
  });

  it('handles negative values', () => {
    expect(formatSize(-2048)).toBe('-2.0 KiB');
  });
});

describe('formatCount', () => {
  it('groups thousands', () => {
    expect(formatCount(0)).toBe('0');
    expect(formatCount(1234567)).toBe('1,234,567');
  });
});

describe('percent', () => {
  it('computes a bounded percentage', () => {
    expect(percent(50, 200)).toBe(25);
    expect(percent(1, 0)).toBe(0);
    expect(percent(-5, 100)).toBe(0);
    expect(percent(200, 100)).toBe(100);
  });
});
