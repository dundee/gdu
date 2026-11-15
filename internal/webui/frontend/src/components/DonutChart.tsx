import { useMemo } from 'react';
import type { Node } from '../types';
import { computeSlices } from '../slices';
import { formatSize, percent } from '../format';

interface DonutChartProps {
  children: Node[];
  apparent: boolean;
  useSIPrefix: boolean;
  hoveredPath: string | null;
  onHover: (path: string | null) => void;
  onSelect: (node: Node) => void;
}

const SIZE = 240;
const CENTER = SIZE / 2;
const RADIUS = 90;
const STROKE = 34;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

export function DonutChart({
  children,
  apparent,
  useSIPrefix,
  hoveredPath,
  onHover,
  onSelect,
}: DonutChartProps) {
  const { slices, total } = useMemo(
    () => computeSlices(children, apparent),
    [children, apparent],
  );

  const hovered = slices.find((s) => s.node?.path === hoveredPath) ?? null;
  const centerPrimary = hovered ? hovered.label : formatSize(total, useSIPrefix);
  const centerSecondary = hovered
    ? `${formatSize(hovered.value, useSIPrefix)} · ${percent(hovered.value, total).toFixed(1)}%`
    : `${slices.length} item${slices.length === 1 ? '' : 's'}`;

  let offsetFraction = 0;

  return (
    <div className="donut" role="img" aria-label="Directory size breakdown">
      <svg viewBox={`0 0 ${SIZE} ${SIZE}`} width="100%" height="100%">
        <g transform={`rotate(-90 ${CENTER} ${CENTER})`}>
          <circle
            cx={CENTER}
            cy={CENTER}
            r={RADIUS}
            fill="none"
            stroke="var(--track)"
            strokeWidth={STROKE}
          />
          {total > 0 &&
            slices.map((slice) => {
              const fraction = slice.value / total;
              const dash = fraction * CIRCUMFERENCE;
              const dashOffset = -offsetFraction * CIRCUMFERENCE;
              offsetFraction += fraction;

              const key = slice.node ? slice.node.path : '__other__';
              const isHovered = hoveredPath !== null && slice.node?.path === hoveredPath;
              const dimmed = hoveredPath !== null && !isHovered;
              const clickable = slice.node?.isDir ?? false;

              return (
                <circle
                  key={key}
                  className="donut-slice"
                  cx={CENTER}
                  cy={CENTER}
                  r={RADIUS}
                  fill="none"
                  stroke={slice.color}
                  strokeWidth={isHovered ? STROKE + 6 : STROKE}
                  strokeDasharray={`${dash} ${CIRCUMFERENCE - dash}`}
                  strokeDashoffset={dashOffset}
                  opacity={dimmed ? 0.35 : 1}
                  style={{ cursor: clickable ? 'pointer' : 'default' }}
                  onMouseEnter={() => slice.node && onHover(slice.node.path)}
                  onMouseLeave={() => onHover(null)}
                  onClick={() => {
                    if (slice.node && slice.node.isDir) {
                      onSelect(slice.node);
                    }
                  }}
                />
              );
            })}
        </g>
      </svg>
      <div className="donut-center">
        <div className="donut-center-primary" title={centerPrimary}>
          {centerPrimary}
        </div>
        <div className="donut-center-secondary">{centerSecondary}</div>
      </div>
    </div>
  );
}
