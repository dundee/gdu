import { useEffect, useMemo, useRef, useState } from 'react';
import type { Node, TreeNode } from '../types';
import { metricValue } from '../slices';
import { formatCount, formatSize, percent } from '../format';
import { layoutTree, type TreeLayoutRect } from './treemap/treeLayout';
import { OTHER_COLOR } from '../colors';
import { deletionStateOf, type Deletion } from '../model';

interface TreeMapProps {
  root: TreeNode;
  apparent: boolean;
  useSIPrefix: boolean;
  colorMap: Map<string, string>;
  hoveredPath: string | null;
  selectedPath: string | null;
  onHover: (path: string | null) => void;
  onSelect: (node: Node) => void;
  // onOpen navigates the app to a directory path: the tile itself when it is
  // a directory, or its immediate parent when it is a file (there is nothing
  // to "open" for a file, so this reveals the directory that contains it).
  onOpen: (path: string) => void;
  // colorFor overrides the location coloring (colorMap) for a projection.
  colorFor?: (rect: TreeLayoutRect) => string;
  // muted tiles are faded, e.g. by a legend filter.
  muted?: (node: Node) => boolean;
  // tooltipDetail adds a line to the hover tooltip.
  tooltipDetail?: (node: Node) => string | null;
  // Items being deleted in the background (hatched) or already deleted
  // (cross-hatched), with everything inside them. They keep their place, so
  // the layout never jumps under the cursor.
  deletions?: ReadonlyMap<string, Deletion>;
  // dimOnHover fades every tile outside the hovered one's ancestry. Off for
  // projections, whose colors carry meaning.
  dimOnHover?: boolean;
}

const DEFAULT_WIDTH = 300;
const DEFAULT_HEIGHT = 200;
const MIN_TILE_AREA = 24;
const MAX_RECTS = 2000;
const DIRECTORY_INSET = 2;

export function TreeMap({
  root,
  apparent,
  useSIPrefix,
  colorMap,
  hoveredPath,
  selectedPath,
  onHover,
  onSelect,
  onOpen,
  colorFor,
  muted,
  tooltipDetail,
  deletions,
  dimOnHover = true,
}: TreeMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: DEFAULT_WIDTH, height: DEFAULT_HEIGHT });

  useEffect(() => {
    const element = containerRef.current;
    if (!element || typeof ResizeObserver === 'undefined') {
      return;
    }
    let raf = 0;
    const observer = new ResizeObserver(([entry]) => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => {
        const { width, height } = entry.contentRect;
        if (width > 0 && height > 0) {
          setSize({ width, height });
        }
      });
    });
    observer.observe(element);
    return () => {
      cancelAnimationFrame(raf);
      observer.disconnect();
    };
  }, []);

  const rects = useMemo(
    () =>
      layoutTree(root, {
        width: size.width,
        height: size.height,
        apparent,
        minArea: MIN_TILE_AREA,
        maxRects: MAX_RECTS,
        inset: DIRECTORY_INSET,
      }),
    [apparent, root, size],
  );
  const total = metricValue(root, apparent);
  const { hoveredNode, outlinedPaths } = useMemo(() => {
    const hoveredRect = rects.find((rect) => rect.node.path === hoveredPath) ?? null;
    const node = hoveredPath === root.path ? root : hoveredRect?.node ?? null;
    const paths = new Set(
      hoveredRect ? [...hoveredRect.ancestorPaths, hoveredRect.node.path] : node ? [root.path] : [],
    );
    return { hoveredNode: node, outlinedPaths: paths };
  }, [rects, hoveredPath, root]);
  const rootHovered = hoveredNode?.path === root.path;

  return (
    <div
      ref={containerRef}
      className={`treemap ${hoveredNode === null ? '' : 'hovering'}`}
      role="img"
      aria-label="Directory size treemap"
    >
      <svg
        viewBox={`0 0 ${size.width} ${size.height}`}
        preserveAspectRatio="none"
        width="100%"
        height="100%"
      >
        <defs>
          <pattern
            id="treemap-pending"
            width="6"
            height="6"
            patternUnits="userSpaceOnUse"
            patternTransform="rotate(45)"
          >
            <rect width="6" height="6" fill="#14171c" />
            <rect width="2" height="6" fill="#3a4150" />
          </pattern>
          <pattern
            id="treemap-deleted"
            width="6"
            height="6"
            patternUnits="userSpaceOnUse"
            patternTransform="rotate(45)"
          >
            <rect width="6" height="6" fill="#14171c" />
            <rect width="1.5" height="6" fill="#4a5262" />
            <rect width="6" height="1.5" fill="#4a5262" />
          </pattern>
          <linearGradient id="treemap-shine" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#fff" stopOpacity="0.28" />
            <stop offset="0.48" stopColor="#fff" stopOpacity="0.04" />
            <stop offset="1" stopColor="#000" stopOpacity="0.3" />
          </linearGradient>
        </defs>
        <rect
          className={`treemap-root ${rootHovered ? 'hovered' : ''}`}
          width={size.width}
          height={size.height}
          fill={OTHER_COLOR}
          stroke={rootHovered ? '#fff' : 'transparent'}
          strokeWidth={rootHovered ? 1.5 : 0}
          onMouseEnter={() => onHover(root.path)}
          onMouseLeave={() => onHover(null)}
        />
        {rects.map((rect) => {
          const { node } = rect;
          const isHovered = node.path === hoveredPath;
          const isAncestor = !isHovered && outlinedPaths.has(node.path);
          const isSelected = node.path === selectedPath;
          const deletion = deletions ? deletionStateOf(deletions, node.path) : null;
          const dimmed =
            (dimOnHover && hoveredNode !== null && !outlinedPaths.has(node.path)) ||
            (muted?.(node) ?? false);
          const showLabel = !node.isDir && rect.w > 34 && rect.h > 16;
          const showSize = showLabel && rect.w > 60 && rect.h > 30;
          const width = Math.max(rect.w - 0.5, 0);
          const height = Math.max(rect.h - 0.5, 0);

          return (
            <g
              key={node.path}
              className={`treemap-tile ${isHovered ? 'hovered' : ''} ${isAncestor ? 'ancestor' : ''} ${isSelected ? 'selected' : ''} ${deletion ?? ''}`}
              aria-busy={deletion === 'pending' || undefined}
              data-deletion={deletion ?? undefined}
              role="button"
              aria-label={`${node.name}, ${formatSize(metricValue(node, apparent), useSIPrefix)}`}
              tabIndex={0}
              style={{ cursor: 'pointer' }}
              opacity={dimmed ? 0.42 : 1}
              onMouseEnter={() => onHover(node.path)}
              onMouseLeave={() => onHover(null)}
              onClick={(event) => {
                event.stopPropagation();
                onSelect(node);
              }}
              onDoubleClick={(event) => {
                event.stopPropagation();
                const targetPath = node.isDir
                  ? node.path
                  : rect.ancestorPaths[rect.ancestorPaths.length - 1];
                if (targetPath) {
                  onOpen(targetPath);
                }
              }}
            >
              <rect
                x={rect.x}
                y={rect.y}
                width={width}
                height={height}
                fill={
                  deletion
                    ? `url(#treemap-${deletion})`
                    : colorFor
                      ? colorFor(rect)
                      : (colorMap.get(rect.colorPath) ?? OTHER_COLOR)
                }
                stroke={
                  isHovered || isAncestor
                    ? '#fff'
                    : isSelected
                      ? '#55b6ff'
                      : 'rgba(0,0,0,0.35)'
                }
                strokeWidth={isHovered ? 1.5 : isAncestor || isSelected ? 1.1 : 0.4}
              />
              {!deletion && (
                <rect
                  className="treemap-shine"
                  x={rect.x}
                  y={rect.y}
                  width={width}
                  height={height}
                  fill="url(#treemap-shine)"
                />
              )}
              {showLabel && (
                <foreignObject
                  x={rect.x + 1.5}
                  y={rect.y + 1}
                  width={Math.max(rect.w - 3, 0)}
                  height={Math.max(rect.h - 2, 0)}
                >
                  <div className="treemap-label">
                    <span className="treemap-label-name">{node.name}</span>
                    {showSize && (
                      <span className="treemap-label-size">
                        {formatSize(metricValue(node, apparent), useSIPrefix)}
                      </span>
                    )}
                  </div>
                </foreignObject>
              )}
            </g>
          );
        })}
      </svg>
      {hoveredNode && (
        <div className="treemap-tooltip">
          <b>{hoveredNode.name}</b>
          <span>{hoveredNode.path}</span>
          <span>
            {formatSize(metricValue(hoveredNode, apparent), useSIPrefix)} ·{' '}
            {percent(metricValue(hoveredNode, apparent), total).toFixed(1)}%
            {hoveredNode.isDir
              ? ` · ${formatCount(hoveredNode.itemCount)} item${hoveredNode.itemCount === 1 ? '' : 's'}`
              : ''}
          </span>
          {deletions && deletionStateOf(deletions, hoveredNode.path) && (
            <span className="treemap-tooltip-detail">
              {deletionStateOf(deletions, hoveredNode.path) === 'pending'
                ? 'Deleting…'
                : 'Deleted; the view updates when you open another directory'}
            </span>
          )}
          {tooltipDetail?.(hoveredNode) && (
            <span className="treemap-tooltip-detail">{tooltipDetail(hoveredNode)}</span>
          )}
        </div>
      )}
    </div>
  );
}
