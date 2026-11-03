import { describe, expect, it } from 'vitest';
import type { Node } from './types';
import {
  AGE_BUCKETS,
  NEUTRAL_COLOR,
  REVIEW_COLOR,
  SAFE_COLOR,
  ageColor,
  ageText,
  candidateShare,
  candidateSummary,
  cleanupColor,
  containsCategory,
  isCategoryNode,
} from './cleanup';

const DAY = 24 * 60 * 60;
const now = 1_800_000_000;

function node(overrides: Partial<Node> = {}): Node {
  return {
    name: 'n',
    path: '/n',
    isDir: true,
    size: 1000,
    usage: 2000,
    itemCount: 1,
    mtime: now,
    ...overrides,
  };
}

describe('cleanupColor', () => {
  it('colors candidates by confidence', () => {
    expect(cleanupColor(node({ confidence: 'safe' }), false)).toBe(SAFE_COLOR);
    expect(cleanupColor(node({ confidence: 'review' }), false)).toBe(REVIEW_COLOR);
  });

  it('keeps items without candidates neutral', () => {
    expect(cleanupColor(node(), false)).toBe(NEUTRAL_COLOR);
  });

  it('tints a directory more the larger its candidate share', () => {
    const little = cleanupColor(node({ categoryTotals: { 'app-cache': { usage: 200, size: 100 } } }), false);
    const lots = cleanupColor(node({ categoryTotals: { 'app-cache': { usage: 1800, size: 900 } } }), false);
    expect(little).not.toBe(NEUTRAL_COLOR);
    expect(lots).not.toBe(little);
    // red channel grows towards SAFE_COLOR's
    expect(parseInt(lots.slice(1, 3), 16)).toBeGreaterThan(parseInt(little.slice(1, 3), 16));
  });
});

describe('candidateShare', () => {
  it('sums categories in the active metric and caps at 1', () => {
    const n = node({
      categoryTotals: {
        'build-artifact': { usage: 500, size: 100 },
        'ml-model': { usage: 500, size: 2000 },
      },
    });
    expect(candidateShare(n, false)).toBe(0.5);
    expect(candidateShare(n, true)).toBe(1);
    expect(candidateShare(node({ usage: 0 }), false)).toBe(0);
  });
});

describe('ageColor and ageText', () => {
  it('maps the latest change to buckets', () => {
    expect(ageColor(now - 5 * DAY, now)).toBe(AGE_BUCKETS[0].color);
    expect(ageColor(now - 90 * DAY, now)).toBe(AGE_BUCKETS[1].color);
    expect(ageColor(now - 300 * DAY, now)).toBe(AGE_BUCKETS[2].color);
    expect(ageColor(now - 900 * DAY, now)).toBe(AGE_BUCKETS[3].color);
    expect(ageColor(0, now)).toBe(NEUTRAL_COLOR);
  });

  it('describes age coarsely', () => {
    expect(ageText(now, now)).toBe('changed today');
    expect(ageText(now - DAY, now)).toBe('unchanged for 1 day');
    expect(ageText(now - 125 * DAY, now)).toBe('unchanged for 4 months');
    expect(ageText(now - 800 * DAY, now)).toBe('unchanged for 2 years');
  });
});

describe('candidateSummary', () => {
  const labels = new Map([
    ['build-artifact', 'Build artifacts'],
    ['ml-model', 'ML models'],
    ['app-cache', 'App caches'],
  ] as const);

  it('lists the largest categories with age', () => {
    const n = node({
      mtime: now - 125 * DAY,
      categoryTotals: {
        'build-artifact': { usage: 2048, size: 2048 },
        'ml-model': { usage: 4096, size: 4096 },
        'app-cache': { usage: 1024, size: 1024 },
      },
    });
    expect(candidateSummary(n, false, false, new Map(labels), now)).toBe(
      '4.0 KiB ml models, 2.0 KiB build artifacts, +1 more · unchanged for 4 months',
    );
  });

  it('is empty without candidates', () => {
    expect(candidateSummary(node(), false, false, new Map(), now)).toBeNull();
  });
});

describe('containsCategory and isCategoryNode', () => {
  it('matches own category or contained totals', () => {
    expect(containsCategory(node({ category: 'ml-model' }), 'ml-model')).toBe(true);
    expect(containsCategory(node({ categoryTotals: { 'ml-model': { usage: 1, size: 1 } } }), 'ml-model')).toBe(true);
    expect(containsCategory(node({ categoryTotals: { 'ml-model': { usage: 1, size: 1 } } }), 'installer')).toBe(false);
  });

  it('recognizes synthetic category nodes', () => {
    expect(isCategoryNode(node({ path: 'category:ml-model' }))).toBe(true);
    expect(isCategoryNode(node())).toBe(false);
  });
});
