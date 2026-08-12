import type { Node } from '../types';

interface BreadcrumbsProps {
  breadcrumbs: Node[];
  onNavigate: (node: Node) => void;
}

export function Breadcrumbs({ breadcrumbs, onNavigate }: BreadcrumbsProps) {
  return (
    <nav className="breadcrumbs" aria-label="Path">
      {breadcrumbs.map((node, i) => {
        const isLast = i === breadcrumbs.length - 1;
        return (
          <span key={node.path} className="crumb">
            {isLast ? (
              <span className="crumb-current">{node.name || node.path}</span>
            ) : (
              <button type="button" className="crumb-link" onClick={() => onNavigate(node)}>
                {node.name || node.path}
              </button>
            )}
            {!isLast && <span className="crumb-sep">/</span>}
          </span>
        );
      })}
    </nav>
  );
}
