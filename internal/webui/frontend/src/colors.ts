// A categorical palette used consistently between the donut chart and the
// table so a slice and its row share the same color. Index into it by child
// position (modulo length); the aggregated "Other" slice uses OTHER_COLOR.

export const PALETTE = [
  '#2479d0',
  '#e67100',
  '#3fb27f',
  '#d0454c',
  '#9b59b6',
  '#f1c40f',
  '#1abc9c',
  '#e84393',
  '#5f8fd0',
  '#c0862d',
  '#7f8c8d',
  '#16a085',
];

export const OTHER_COLOR = '#4a5568';

export function colorAt(index: number): string {
  return PALETTE[index % PALETTE.length];
}
