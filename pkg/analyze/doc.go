// Package analyze scans directories into trees of fs.Item, the way gdu does.
//
// A scan builds the tree; UpdateStats then computes the totals of every
// directory, counting each hard-linked file once:
//
//	analyzer := analyze.CreateAnalyzer()
//	opts := analyze.Options{FollowSymlinks: true}
//	tree := analyzer.AnalyzeDir(ctx, "/home", opts)
//	tree.UpdateStats(opts.FilteringFiles())
//	for entry := range tree.GetFiles(fs.SortBySize, fs.SortDesc) {
//		fmt.Println(entry.GetName(), entry.GetUsage())
//	}
//
// CreateAnalyzer builds the whole tree in memory, CreateSeqAnalyzer does the
// same reading one directory at a time, and CreateTopDirAnalyzer keeps only
// the totals of the entries of the scanned directory, which takes far less
// memory. Cancelling ctx stops a scan, which returns what it found so far;
// GetProgress tells how far a running scan has got.
//
// The analyzers of packages analyze/badger and analyze/sqlite keep their
// results in a database, from which a later run can load them again with
// LoadAnalysis. They are packages of their own, so that a program not using
// them does not link the databases.
//
// A tree is not safe for concurrent use: whoever reads it while another
// goroutine changes it must synchronise the two.
package analyze
