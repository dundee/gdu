# Architecture TODO — Deepening Opportunities

Prioritized list of refactors that turn **shallow modules** (interface nearly as
complex as implementation) into **deep** ones — a lot of behaviour behind a small
interface, placed at a clean **seam**, testable through that interface. Ordered by
leverage (payback across call sites and tests) weighted toward recently-changed
hot spots (`pkg/analyze`, `tui`, `stdout`, `report`).

Vocabulary: _module, interface, depth, seam, adapter, leverage, locality_ (see the
`codebase-design` skill). The **deletion test**: if deleting a module makes
complexity vanish it was a pass-through; if complexity reappears across N callers
it was earning its keep.

No `CONTEXT.md` or ADRs exist yet, so the code's own domain terms are used:
_analyzer_, _Item_, _Dir_, _storage strategy_, _hard-link tracker_.

---

## P0 — One entry-resolution module behind a storage-strategy seam

**Strength: Strong**

- **Files:** `pkg/analyze/parallel.go:30`, `sequential.go:25`, `parallel_stable.go:25`,
  `stored.go:33`, `parallel_top_dir.go:40`, `sqlite.go:896` (six `AnalyzeDir` +
  `processDir` implementations); prototype already in `sqlite.go:983` (`processFile`)
  and `sqlite.go:948` (`fileStat`).
- **Problem:** Six analyzers each re-implement the same per-entry pipeline
  (`os.ReadDir` → filter → `f.Info()` → follow symlink → time filter → archive branch
  → build item + flag → platform stat). ~700 duplicated lines. The duplication is
  already breeding drift bugs:
  - `stored.go:152` handles `.zip` archives but **silently drops `.tar` support** that
    `parallel.go:161` and `sequential.go:134` have.
  - `totalUsage` (`parallel.go:190`) vs `totalSize` (`sequential.go:163`) name the same
    value inconsistently.
  - Only `parallel.go` and `sequential.go` call `setCurrentDir`, so scan **preview**
    (`GetCurrentDir`) is silently broken for the other four analyzers.
- **Solution:** Extract the per-`os.DirEntry` pipeline into **one deep function**
  returning a resolved item + stats (generalize `sqlite.go`'s `processFile`/`fileStat`).
  Each analyzer becomes a thin **storage-strategy adapter** (~30 lines) that only decides
  _how to store_ the result: append to `Dir.Files`, send over a channel, insert into
  badger, insert into SQLite, aggregate into `TopDir`.
- **Deletion test:** deleting the shared resolver concentrates the traversal complexity
  back into six places → it earns its keep. Strong signal.
- **Benefits:** _Locality_ — symlink/archive/time-filter/hard-link logic fixed once,
  fixed everywhere; the tar-in-stored and preview divergences disappear structurally.
  _Leverage_ — the resolver is the one place tests exercise the whole pipeline.
  _Testability_ — the resolver is a pure-ish function testable without standing up an
  analyzer; storage adapters test in isolation.

---

## P1 — Consolidate size/count formatting into one `common` module

**Strength: Strong**

- **Files:** `stdout/stdout.go:596` & `:618`; `report/export.go:328` & `:350`;
  `tui/format.go:281` & `:302` (size); `stdout/stdout.go:564` & `tui/format.go:263`
  (count). Constants already shared in `internal/common/ui.go`.
- **Problem:** The binary-prefix and decimal-prefix formatting cascades are copied
  **byte-for-byte three times**; `formatSize`/`formatCount` wrappers likewise. The only
  difference is how color is applied (color object vs inline markup). ~180 duplicated
  lines. `report/export.go`'s copies have **no direct format test** — only golden files.
- **Solution:** One deep `common.FormatSize(size, opts)` / `common.FormatCount(count)`
  returning a plain string; callers apply color. This is the target state the codebase
  already reached for top-N (`analyze.CollectTopFiles` is shared by stdout/export).
- **Deletion test:** deleting the shared formatter reappears as three cascades → keeps.
- **Benefits:** _Locality_ — a unit change (new prefix, rounding fix) lands once.
  _Testability_ — one formatter with one table-driven test replaces three partial ones
  and closes the untested `report` path.

---

## P2 — Collapse the empty-directory size to a single constant — **mostly done**

**Strength: Strong** (cheap, high-correctness)

- **Files:** `pkg/analyze/file.go` — `EmptyDirSize`, `dirTotals`, `resolveDirStats` and
  `aggregateDirEntries` are now the single definition of directory accounting.
  `Dir.updateStats`, `StoredDir.updateStats` and `SqliteAnalyzer.processDir` all route
  through it; `parallel_top_dir.go` and `top_dir.go` already used the constant.
- **Was:** raw `4096` hardcoded as the directory's own size _and usage_ in
  `stored.go` and `sqlite.go`, so `--db` scans charged every directory 4096 bytes of disk
  usage that the in-memory analyzer did not. An empty directory reported 4096/4096 instead
  of 512/0, and the error compounded up the tree. Fixed, with
  `TestEmptyDirStatsAcrossAnalyzers` locking parity across all five analyzers.
- **Remaining:** `zipdir.go:164-165` and `tardir.go:241-242` still seed synthetic archive
  directories with `Size: 4096, Usage: 4096`. These are dead values — `Dir.updateStats`
  overwrites them — but they are misleading and should be dropped or set from
  `EmptyDirSize`.
- **Benefits:** _Locality_ — one number, one place; removes silent per-analyzer divergence.

---

## P3 — Split the 24-method `fs.Item` interface

**Strength: Worth exploring**

- **Files:** `pkg/fs/file.go:31-54` (24 methods). Implementations that panic on most of it:
  `analyze/top_dir.go:76-86,122,157-162` (`SimpleDir`), `analyze/stored.go:444`
  (`ParentDir`), `analyze/file.go:157-174` (`File` mutation methods),
  `analyze/sqlite.go` (`SqliteItem`).
- **Problem:** A **fat interface** forces shallow types to satisfy it by
  `panic("not implemented")`. The interface (the full test surface) is far wider than any
  single consumer needs; export/stdout only read.
- **Solution:** Segment into a read-only core (`GetSize`/`GetName`/`GetFiles`/…), a
  `Directory` facet (children + mutation: `AddFile`/`RemoveFile`/`SetParent`), `Encodable`
  (`EncodeJSON`), and `Lockable` (`RLock`). Consumers depend on the narrow facet they use.
- **Deletion test:** partial — the panics are a symptom, not a pass-through; the win is
  narrower seams, not deleted code.
- **Benefits:** _Leverage_ — export/stdout depend on a small read interface. _Testability_
  — the "preview only works on 2 of 6 analyzers" class of gap becomes a compile-time fact;
  the panicking stubs disappear. Larger, riskier change — sequence it after P0.

---

## P4 — Consolidate hard-link "count once" into one tracker module

**Strength: Worth exploring**

- **Files:** `analyze/file.go:108` (`alreadyCounted`, package-global `fileFlagMu`),
  `analyze/parallel_top_dir.go:237` (`sync.Map`), `analyze/sqlite.go:1028`
  (`hasInodeLocked`), plus the badger path via `Dir.updateStats`.
- **Problem:** "Count each inode once" is implemented **four times** with four different
  mechanisms; they can (and do) drift on symlink-target recording. `fileFlagMu`
  (`file.go:17`) is a single process-global `RWMutex` guarding every `File.Flag` — a global
  serialization point under parallel scans.
- **Solution:** One `HardLinkTracker` module (interface: "have I seen this inode?") injected
  into the shared resolver from P0. Removes the global mutex coupling.
- **Benefits:** _Locality_ + _Testability_ — hard-link semantics are one testable module,
  not an emergent property spread across four files. Best done together with P0.

---

## P5 — Introduce a `Walker` seam for depth-limit / summarize / top-N

**Strength: Worth exploring**

- **Files:** depth-limit `stdout/stdout.go:410` vs `report/export.go:174`; summarize
  `stdout/stdout.go:301` vs `report/export.go:209`; top-N already shared via
  `analyze/top.go:33`.
- **Problem:** Each output backend re-codes tree traversal + depth-limiting + summarize on
  top of the raw `Item` tree. JSON encoding is model-owned (`Item.EncodeJSON`) while text
  formatting is UI-owned — inconsistent placement of "turn an Item into output". SQLite even
  duplicates JSON encoding a 4th time (`sqlite.go:642`).
- **Solution:** A `Walker`/`Formatter` seam — walk once, emit per node via a sink. stdout,
  JSON export, TUI, and web differ only in the per-node sink.
- **Benefits:** _Leverage_ — one traversal reused by four backends. _Locality_ — depth/
  summarize semantics fixed once. Depends conceptually on P3 (narrow read interface).

---

## P6 — Break `report`'s concrete `analyze` coupling

**Strength: Worth exploring**

- **Files:** `report/import.go:15` returns `*analyze.Dir`; `report/export.go:162,175,216`
  type-assert `dir.(*analyze.Dir)` and `f.(*analyze.File)`.
- **Problem:** The `report` layer reaches through the `fs.Item` abstraction to concrete
  `analyze` types (layering inversion), so export transforms can only be tested by building
  real `analyze` trees.
- **Solution:** Have `import.ReadAnalysis` return `fs.Item` and drive export transforms
  through the interface (enabled by P3's read facet).
- **Benefits:** _Testability_ — transforms testable with fakes; removes the casts.

---

## P7 — Shrink the `app.UI` interface / give export a sub-interface

**Strength: Worth exploring**

- **Files:** `cmd/gdu/app/app.go:33-52` (19-method `UI`); `report/export.go:76-96`
  (five no-op stub methods only present to satisfy it: `StartUILoop`, `SetCollapsePath`,
  `SetShowSymlinkTarget`, `ListDevices`, `ReadAnalysis`).
- **Problem:** A fat `app.UI` interface forces the export backend to carry stubs it never
  uses — pure interface tax.
- **Solution:** Split `app.UI` so export depends on the subset it implements.
- **Benefits:** _Locality_ — no dead stubs; each backend's interface is its real surface.

---

## P8 — Remove global mutable state in output modules

**Strength: Worth exploring**

- **Files:** `color.NoColor` set as a package-global side effect in `report/export.go:69`
  and `stdout/stdout.go:92`; `progressRunes`/`progressRunesCount` package vars
  `stdout/stdout.go:40-44` mutated by `UseOldProgressRunes()`; `stdout.AnalyzePath` calls
  `os.Stat` directly (`stdout/stdout.go:206`) with no injected path-checker (contrast the
  injectable `PathChecker` in `cmd/gdu/app/app.go:206`).
- **Problem:** Constructing one UI mutates process-wide flags affecting the other and all
  tests → order-dependent, non-parallelizable tests.
- **Solution:** Make color/progress-runes per-UI fields; inject a `PathChecker` into
  `stdout.UI`.
- **Benefits:** _Testability_ — no order-sensitive globals; color/path behaviour injectable.

---

## P9 — Decompose the TUI `UI` god object

**Strength: Speculative** (large, invasive; do last)

- **Files:** `tui/tui.go:28-113` — `UI` has **84 fields** and **127 methods** (36 exported)
  across 15 files; embeds `common.UI` (13 more fields). Deletion-related logic spans 5 files
  with the `action → deleteFun` switch copied 4× (`actions.go:236`, `background.go:35`,
  `marked.go:58`, plus confirm dialogs). `formatFileRow` ≈ `formatCollapsedRow`
  (`tui/format.go:43` vs `:152`, ~80% identical). `ui.app.(*tview.Application)` force-cast
  (`keys.go:157`) defeats the `TermApplication` mock, making Ctrl-Z untestable.
- **Problem:** Widget refs, config, injected deps, live model, concurrency primitives, and
  ephemeral view state all sit on one struct. Almost nothing is testable except by driving a
  real tview widget tree and manually pumping the mocked draw queue.
- **Solution (incremental):** Peel cohesive sub-modules off the god object one seam at a
  time — a `Deletion` module (one `action → deleteFun` mapping used by sync/async/marked),
  a shared row-builder for `formatFileRow`/`formatCollapsedRow`, a config struct for the
  ~25 one-line setters (`tui.go:251-495`). Route Ctrl-Z through the `TermApplication`
  interface.
- **Benefits:** _Locality_ + _Testability_ — deletion and row-formatting become testable
  without a screen. Marked Speculative because the god object is load-bearing and the
  payoff is gradual; tackle the small internal seams first, not a big-bang rewrite.

---

## Top recommendation

Start with **P0 (shared entry-resolution + storage-strategy seam)**. It is the highest-
leverage move by a wide margin: it collapses ~700 duplicated lines across six analyzers,
and it structurally eliminates three already-live bugs (missing `.tar` in stored, the
`totalUsage`/`totalSize` drift, and preview being broken for four of six analyzers) rather
than patching them one by one. `sqlite.go`'s `processFile`/`fileStat` already sketches the
deep function, so the design risk is low. P2 (empty-dir constant) and P4 (hard-link tracker)
naturally fold into the same change, and P1 (formatting) is an independent, cheap Strong win
that can proceed in parallel.
