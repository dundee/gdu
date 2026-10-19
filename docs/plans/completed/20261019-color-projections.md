# Cleanup Projections for the Web UI Treemap

## Overview

The web UI treemap colors every rectangle by its top-level ancestor, so color
carries no meaning, and small files disappear into aggregated directory blocks
(the tree payload is capped at 5000 nodes and rectangles below `minArea` are not
drawn). Users who want to free disk space have to walk the tree by hand to find
regenerable data.

This feature adds **projections**: alternative colorings (and, in phase 2, an
alternative tree shape) over the same treemap that draw attention to
**cleanup candidates** without deleting anything on their own.

- **Phase 1**
  - server-side classifier that assigns a category and confidence to items,
    preferring marker-based rules (`node_modules` only next to `package.json`)
    over bare name matches, plus a `.gitignore` signal;
  - per-directory bytes-by-category aggregated over the **full** tree, so
    hidden small items still count;
  - "Cleanup" projection: non-candidates grey, candidates in one accent hue
    whose strength follows confidence;
  - "Age" projection: color by latest mtime;
  - legend with per-category totals that doubles as a highlight filter;
  totals, legend and pivot are always scoped to the **current directory**
  (the one the user drilled into), not the whole scan;
  - user rules and exclusions in `~/.gdu.yaml`.
- **Phase 2**
  - "By category" projection: the tree is pivoted to category → directories,
    so cache bytes scattered over many small directories become one visible
    block;
  - optional lightweight multi-select with "will free X" and batch
    move-to-trash, reusing the existing confirm modal;
  - working trash on macOS.

Out of scope: duplicate detection (too expensive for the value).

The color channel is never used for size: area already encodes size.

## Context (from discovery)

- Tree payload: `webui/tree.go` (`nodeJSON`, `treeJSON`, `buildTree`, `maxTreeNodes = 5000`).
- Routes: `webui/server.go` (`/api/v1/nodes`, `/api/v1/tree`, `DELETE /api/v1/nodes?mode=trash`).
- Delete/trash handler: `webui/handlers.go` (`deleteModeTrash`, `ui.trasher`).
- Treemap layout and coloring: `webui/frontend/src/components/treemap/treeLayout.ts`
  (`colorPath` inherited from top-level child), `TreeMap.tsx`, `TreeMapView.tsx`.
- Palette: `webui/frontend/src/colors.ts`.
- View state (`'donut' | 'treemap'`): `webui/frontend/src/model.tsx`, `App.tsx`.
- Config: `cmd/gdu/app/app.go` (`Flags` with `yaml` tags, `~/.gdu.yaml`), webui
  constructed via `webui.CreateUI` in `app.go`.
- Existing `-G/--ignore-from-gitignore` (`internal/common/gitignore.go`) translates
  one gitignore file into scan-ignore regexes; it is not a "is this path ignored
  in its repo" matcher, so it cannot be reused directly.
- Trash: `pkg/remove/trash_darwin.go` is a stub that errors; ADR 0002 describes
  the external trash command escape hatch. `/usr/bin/trash` ships with current macOS.
- Item mtime is already in the payload (`Mtime`); directory mtime is the latest
  mtime inside it.
- Glossary: `CONTEXT.md`; decisions: `docs/adr/`.
- Tests: `make test` (gotestsum), `make test-web` (vitest), `make check` (lint + both).

## Development Approach

- **testing approach**: Regular (code first, then tests in the same task)
- complete each task fully before moving to the next
- make small, focused changes
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task
  - write unit tests for new and modified functions
  - cover both success and error scenarios
- **CRITICAL: all tests must pass before starting next task** - no exceptions
- **CRITICAL: update this plan file when scope changes during implementation**
- no backward compatibility constraints: change the API payload and frontend
  types freely

## Testing Strategy

- **Go unit tests**: table-driven tests for rule matching, marker checks,
  gitignore signal, aggregation, config parsing, handlers (`httptest`).
- **Frontend unit tests** (vitest): color functions per projection, legend
  filter, pivot layout, multi-select totals and batch trash flow (mocked `api.ts`).
- No browser e2e suite exists in the project; UI behavior is covered by vitest
  component tests alongside existing ones (`App.test.tsx` etc.).

## Progress Tracking

- mark completed items with `[x]` immediately when done
- add newly discovered tasks with ➕ prefix
- document issues/blockers with ⚠️ prefix
- keep plan in sync with actual work done

## What Goes Where

- **Implementation Steps** (`[ ]`): code, tests, docs in this repo.
- **Post-Completion**: manual checks on a real disk, upstream PR.

## Implementation Steps

### Phase 1

### Task 1: Classification rules and matcher

**Files:**
- Create: `pkg/cleanup/rules.go`
- Create: `pkg/cleanup/classify.go`
- Create: `pkg/cleanup/classify_test.go`

- [ ] define `Category` (`build-artifact`, `package-cache`, `system-cache`, `ml-model`, `installer`, `git-ignored`) and `Confidence` (`safe`, `review`) with label and hint (e.g. "remove with `ollama rm`")
- [ ] define `Rule` (category, confidence, directory-name or path glob or file extension, optional sibling marker files, optional hint)
- [ ] embed default rule table: `node_modules`+`package.json`, `target`+`Cargo.toml`, `.next`/`dist`/`build` only with a project marker, `__pycache__`, `.venv`+`pyproject.toml|requirements.txt`, `.gradle`, `DerivedData`, `~/Library/Caches`, `~/.cache`, `~/.npm`, `go/pkg/mod/cache`, `~/.cargo/registry`; `~/.ollama/models` and `~/.cache/huggingface` as `ml-model`/`review`; `*.gguf`, `*.safetensors`, `*.onnx`, `*.ckpt` as `ml-model`/`review`; `*.dmg`, `*.pkg`, `*.iso` and archives under `~/Downloads` as `installer`/`review`
- [ ] implement `Classifier.Classify(item fs.Item) (Match, bool)`: a matched directory is a unit (descendants inherit, not re-matched); marker check looks up siblings in the in-memory tree, no disk access
- [ ] support exclusions: an excluded path and everything below it never matches
- [ ] write tests: marker present vs absent, nested match inherits, extension rules, `~` expansion, exclusions
- [ ] run `make test` - must pass before next task

### Task 2: `.gitignore` signal

**Files:**
- Create: `pkg/cleanup/gitignore.go`
- Create: `pkg/cleanup/gitignore_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] pick a maintained gitignore matcher (candidate: `github.com/go-git/go-git/v5/plumbing/format/gitignore`, which handles nested `.gitignore` files); avoid a custom matcher
- [ ] detect repository roots from `.git` entries in the scanned tree; read their `.gitignore` files from disk once per classification pass
- [ ] an ignored item that matched no rule gets `git-ignored`/`review`; a rule match keeps its own category but `review` is upgraded to `safe` when also git-ignored
- [ ] items outside any repository are unaffected
- [ ] write tests with a temp repo: ignored dir, negated pattern, nested `.gitignore`, no repo
- [ ] run `make test` - must pass before next task

### Task 3: Full-tree aggregation

**Files:**
- Create: `pkg/cleanup/aggregate.go`
- Create: `pkg/cleanup/aggregate_test.go`

- [ ] implement one post-order pass over the full tree producing, per directory, bytes by category for both disk usage and apparent size, plus global totals
- [ ] store results in a map keyed by `fs.Item`; hard-linked files counted the way gdu already counts them
- [ ] write tests: mixed directory, unit directory, empty tree, apparent vs usage
- [ ] run `make test` - must pass before next task

### Task 4: Config for rules and exclusions

**Files:**
- Modify: `cmd/gdu/app/app.go`
- Modify: `cmd/gdu/app/app_test.go`
- Modify: `webui/webui.go`

- [ ] add `cleanup:` section to `Flags`: `exclude: []string`, `rules: []` (same fields as `Rule`); user rules are evaluated before defaults
- [ ] validate rules at startup (unknown category, empty matcher) and fail with a clear error
- [ ] pass the built `Classifier` into `webui.CreateUI`
- [ ] write tests: valid config, invalid category, exclusion with `~`
- [ ] run `make test` - must pass before next task

### Task 5: Expose classification in the API

**Files:**
- Modify: `webui/tree.go`
- Modify: `webui/server.go`
- Modify: `webui/handlers.go`
- Modify: `webui/webui_test.go`

- [ ] compute classification + aggregation once after the scan finishes; recompute after a successful delete/trash
- [ ] extend `nodeJSON` with `category`, `confidence`, `hint` (unit matches only) and, for directories, `categoryUsage` / `categorySize` maps
- [ ] add `GET /api/v1/categories?path=<dir>`: category list with label, confidence, hint and totals for that directory's subtree (read from the cached per-directory aggregation; no `path` = scan root); unknown path → 404, as `/api/v1/nodes`
- [ ] return 503 (or `pending`) from `/api/v1/categories` while scanning, consistent with existing status handling
- [ ] write handler tests: payload fields, totals for root and a nested `path`, unknown path, recompute after delete, during scan
- [ ] run `make test` - must pass before next task

### Task 6: Projection switch and color functions

**Files:**
- Modify: `webui/frontend/src/types.ts`
- Modify: `webui/frontend/src/api.ts`
- Modify: `webui/frontend/src/model.tsx`
- Modify: `webui/frontend/src/colors.ts`
- Modify: `webui/frontend/src/components/treemap/treeLayout.ts`
- Modify: `webui/frontend/src/components/TreeMap.tsx`
- Modify: `webui/frontend/src/components/Header.tsx`
- Create: `webui/frontend/src/colors.test.ts`

- [ ] add `projection: 'default' | 'cleanup' | 'age'` to the model, with a header control and a keyboard shortcut
- [ ] `cleanup` color: unit match → accent hue (`safe` strong, `review` light); directory without own match → tinted by its candidate share (`sum(categoryUsage) / usage`); everything else neutral grey; excluded paths grey
- [ ] `age` color: sequential scale over latest mtime (e.g. < 1 month, < 6 months, < 1 year, older)
- [ ] tooltip line for candidates, e.g. "14 GB of build cache, unchanged for 4 months" plus hint
- [ ] colors must pass contrast in light and dark themes
- [ ] write tests for color functions and tooltip text
- [ ] run `make test-web` - must pass before next task

### Task 7: Legend with totals and highlight filter

**Files:**
- Create: `webui/frontend/src/components/CategoryLegend.tsx`
- Create: `webui/frontend/src/components/CategoryLegend.test.tsx`
- Modify: `webui/frontend/src/components/TreeMapView.tsx`
- Modify: `webui/frontend/src/components/HelpModal.tsx`

- [ ] show legend in `cleanup` projection for the current directory: category, total size, share of the directory's size; refetch on navigation
- [ ] click a category to dim every rectangle that does not contain it; click again to clear
- [ ] document projections and shortcut in the help modal
- [ ] write tests: totals rendered, totals change after drilling into a subdirectory, filter toggling dims rectangles
- [ ] run `make test-web` - must pass before next task

### Phase 2

### Task 8: Pivoted "by category" tree

**Files:**
- Modify: `webui/tree.go`
- Modify: `webui/server.go`
- Modify: `webui/webui_test.go`

- [ ] add `GET /api/v1/tree?path=<dir>&pivot=category`: virtual root (the current directory) → one virtual node per category → topmost matched directories/files inside `<dir>` with their real paths, using the same 5000-node budget
- [ ] virtual category nodes have no path and are not deletable (same rule as the virtual top-level directory, ADR 0002-multiple-scanned-roots)
- [ ] write tests: pivot shape at root and at a nested `path`, budget, virtual nodes rejected by delete
- [ ] run `make test` - must pass before next task

### Task 9: "By category" projection in the frontend

**Files:**
- Modify: `webui/frontend/src/model.tsx`
- Modify: `webui/frontend/src/api.ts`
- Modify: `webui/frontend/src/components/TreeMapView.tsx`
- Modify: `webui/frontend/src/components/treemap/treeLayout.ts`
- Modify: `webui/frontend/src/components/treemap/treeLayout.test.ts`

- [ ] add `categories` projection that loads the pivot tree for the current directory; top-level blocks colored per category, children inherit; navigating reloads it
- [ ] clicking a real node shows its real path in breadcrumbs/tooltip; reveal works as in the normal tree
- [ ] write tests: layout over pivot tree, virtual node not actionable
- [ ] run `make test-web` - must pass before next task

### Task 10: macOS trash

**Files:**
- Modify: `pkg/remove/trash_darwin.go`
- Modify: `pkg/remove/trash_darwin_test.go`
- Create: `docs/adr/0003-macos-trash-uses-system-trash-command.md`

- [ ] implement `MoveItemToTrash` on darwin via `/usr/bin/trash` (exec without a shell, path as an argument); clear error when the binary is missing, pointing at the external trash command option
- [ ] record the decision in an ADR
- [ ] write tests: successful move of a temp file, missing binary error
- [ ] run `make test` - must pass before next task

### Task 11 (optional, lightweight): Multi-select and batch trash

Existing single-item delete/trash from the treemap stays the primary flow. Do
this task only if it stays small; otherwise drop it and note it here.

**Files:**
- Modify: `webui/frontend/src/model.tsx`
- Modify: `webui/frontend/src/useItemActions.ts`
- Modify: `webui/frontend/src/components/TreeMap.tsx`
- Modify: `webui/frontend/src/components/DeleteConfirmModal.tsx`

- [ ] modifier-click toggles a node into a selection set in the model; selecting a directory drops its selected descendants
- [ ] the existing confirm modal lists the selection and "will free X" with the current metric
- [ ] confirm sends the existing `DELETE /api/v1/nodes?mode=trash` per item sequentially, reports per-item failures, then refreshes tree and totals
- [ ] respect `--no-delete` and the existing action token
- [ ] write tests: nested selection dedup, total, partial failure
- [ ] run `make test-web` - must pass before next task

### Task 12: Verify acceptance criteria

- [ ] verify every Overview item is implemented for both phases
- [ ] verify edge cases: multi-root scans, archives/virtual nodes, stopped scan, deletion recompute
- [ ] run full suite: `make check`
- [ ] rebuild embedded bundle: `make build-web`

### Task 13: [Final] Update documentation

- [ ] add glossary terms to `CONTEXT.md`: Projection, Cleanup candidate, Category, Confidence
- [ ] document `cleanup:` config and projections in `README.md`; regenerate man page if flags changed
- [ ] move this plan to `docs/plans/completed/`

## Technical Details

**Config (`~/.gdu.yaml`):**

```yaml
cleanup:
  exclude:
    - ~/work/current-project
  rules:
    - category: build-artifact
      confidence: safe
      dir: .gradle
      markers: [build.gradle, build.gradle.kts]
    - category: ml-model
      confidence: review
      path: ~/models
      hint: "Re-downloadable from the model hub"
```

**Payload additions (`nodeJSON`):**

```json
{
  "category": "build-artifact",
  "confidence": "safe",
  "hint": "",
  "categoryUsage": { "build-artifact": 1503238553, "ml-model": 0 },
  "categorySize": { "build-artifact": 1499999999 }
}
```

`category` is set only on unit matches; `categoryUsage`/`categorySize` only on
directories and only for non-zero categories.

**`GET /api/v1/categories?path=<dir>`** (totals are for `<dir>`'s subtree):

```json
{
  "total": { "usage": 500107862016, "size": 498000000000 },
  "categories": [
    { "id": "build-artifact", "label": "Build artifacts", "confidence": "safe",
      "hint": "", "usage": 24696061952, "size": 24000000000 }
  ]
}
```

**Processing flow:** scan finishes → classify (rules, markers from the in-memory
tree, `.gitignore` files read from disk for detected repos) → aggregate post-order
→ cache → serve; delete/trash → recompute.

**Classification precedence:** exclusion → user rules → default rules →
`.gitignore` signal. First match on the way down wins; descendants of a unit
match are not classified again.

## Post-Completion

**Manual verification:**
- run `gdu --web ~` on a real macOS home with Node/Rust/Python projects, Ollama
  and Hugging Face caches; check false positives (`dist/` committed to a repo,
  `build/` with no marker)
- check the pivot projection makes scattered caches visible
- trash several selected items and restore one from Finder's Trash
- check performance of classification on a multi-million-item scan

**External:**
- open an upstream PR to `dundee/gdu` after the feature settles on the fork
