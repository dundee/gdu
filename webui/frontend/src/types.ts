// Wire types shared with the Go backend (see webui/handlers.go and webui/tree.go).

export interface Node {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  usage: number;
  itemCount: number;
  mtime: number;
  flag?: string;
  // Set when the item is a cleanup candidate on its own (see pkg/cleanup).
  category?: Category;
  confidence?: Confidence;
  hint?: string;
  // Space taken by cleanup candidates within the item, per category.
  categoryTotals?: Partial<Record<Category, Amount>>;
}

export type Category =
  | 'build-artifact'
  | 'package-cache'
  | 'app-cache'
  | 'toolchain'
  | 'ml-model'
  | 'installer'
  | 'git-ignored';

export type Confidence = 'safe' | 'review';

export interface Amount {
  usage: number;
  size: number;
}

export interface CategoryTotal extends Amount {
  id: Category;
  label: string;
}

// GET /api/v1/categories: candidates within one directory.
export interface CategoriesResponse {
  total: Amount;
  categories: CategoryTotal[];
}

export interface TreeNode extends Node {
  children: TreeNode[];
}

export interface NodeResponse {
  node: Node;
  breadcrumbs: Node[];
  children: Node[];
}

export interface Progress {
  currentItem: string;
  itemCount: number;
  totalUsage: number;
}

export type ScanState = 'scanning' | 'done' | 'error';

export interface Status {
  state: ScanState;
  error?: string;
  rootPath: string;
  progress: Progress;
  showApparentSize: boolean;
  showRelativeSize: boolean;
  useSIPrefix: boolean;
  deleteAllowed: boolean;
}

export type SortKey = 'size' | 'name' | 'itemCount' | 'mtime';
export type SortOrder = 'asc' | 'desc';
