import type { Progress } from '../types';
import { formatCount, formatSize } from '../format';

interface ProgressBarProps {
  progress: Progress;
  useSIPrefix: boolean;
}

export function ProgressBar({ progress, useSIPrefix }: ProgressBarProps) {
  return (
    <div className="scanning">
      <div className="spinner" />
      <h2>Scanning…</h2>
      <div className="scan-stats">
        <span>{formatCount(progress.itemCount)} items</span>
        <span>{formatSize(progress.totalUsage, useSIPrefix)}</span>
      </div>
      <div className="scan-current" title={progress.currentItem}>
        {progress.currentItem}
      </div>
    </div>
  );
}
