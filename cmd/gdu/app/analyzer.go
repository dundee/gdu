package app

import (
	"fmt"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/pkg/analyze"
	"github.com/dundee/gdu/v6/pkg/analyze/badger"
	"github.com/dundee/gdu/v6/pkg/analyze/sqlite"
)

// analyzer returns the analyzer of the run, which is the one place it is
// chosen: the database of --db, a sequential one with --sequential (validate
// refuses both at once), and otherwise a parallel one. stdout without --top
// or --depth only lists the entries of the scanned directory, so it keeps
// just their totals.
func (a *App) analyzer(front frontend, s *config.Settings) (analyze.Analyzer, error) {
	switch {
	case a.Flags.DbPath != "":
		return a.databaseAnalyzer()
	case a.Flags.SequentialScanning:
		return analyze.CreateSeqAnalyzer(), nil
	case front == frontendStdout && s.Display.Top == 0 && s.Display.Depth == 0:
		return analyze.CreateTopDirAnalyzer(), nil
	}
	return analyze.CreateAnalyzer(), nil
}

// databaseAnalyzer returns the analyzer storing its results in --db, which
// is replaced first when this run scans into it.
func (a *App) databaseAnalyzer() (analyze.Analyzer, error) {
	if err := checkDatabase(a.Flags.DbPath); err != nil {
		return nil, err
	}
	if a.replacesDatabase() {
		if err := removeDatabase(a.Flags.DbPath); err != nil {
			return nil, err
		}
	}
	if isBadgerPath(a.Flags.DbPath) {
		return badger.CreateAnalyzer(a.Flags.DbPath), nil
	}
	analyzer, err := sqlite.CreateAnalyzer(a.Flags.DbPath)
	if err != nil {
		return nil, fmt.Errorf("creating sqlite analyzer: %w", err)
	}
	return analyzer, nil
}
