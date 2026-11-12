package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dundee/gdu/v5/pkg/selfupdate"
)

const selfUpdateFlag = "--self-update"

// SelfUpdater replaces the running binary with the latest release.
type SelfUpdater interface {
	Update(ctx context.Context) (selfupdate.Result, error)
}

// SelfUpdateOutcome tells the caller what to do after RunSelfUpdate.
type SelfUpdateOutcome int

const (
	// SelfUpdateExit means there is nothing more to do.
	SelfUpdateExit SelfUpdateOutcome = iota
	// SelfUpdateContinue means the normal run continues with the current binary.
	SelfUpdateContinue
	// SelfUpdateRestart means the new binary is run with the remaining arguments.
	SelfUpdateRestart
)

// RunSelfUpdate updates gdu and decides what happens next. args are the
// command line arguments without the program name and without --self-update.
//
// When --self-update was the only argument, gdu exits afterwards. Otherwise the
// requested command still runs: with the new binary when the update succeeded,
// with the current one when there was nothing to update or the update failed.
func RunSelfUpdate(ctx context.Context, w io.Writer, u SelfUpdater, args []string) (SelfUpdateOutcome, error) {
	onlyUpdate := len(args) == 0

	fmt.Fprintln(w, "Checking for a new version of gdu...")
	res, err := u.Update(ctx)
	switch {
	case err != nil && onlyUpdate:
		return SelfUpdateExit, fmt.Errorf("self-update: %w", err)
	case err != nil:
		fmt.Fprintf(w, "Self-update failed: %s\nContinuing with the current version.\n", err)
		return SelfUpdateContinue, nil
	case !res.Updated:
		fmt.Fprintf(w, "gdu %s is the latest version\n", res.CurrentVersion)
		if onlyUpdate {
			return SelfUpdateExit, nil
		}
		return SelfUpdateContinue, nil
	}

	fmt.Fprintf(w, "Updated gdu %s -> %s\n", res.CurrentVersion, res.LatestVersion)
	if onlyUpdate {
		return SelfUpdateExit, nil
	}
	return SelfUpdateRestart, nil
}

// StripSelfUpdateFlag removes --self-update from args, so that the restarted
// binary does not update again. Arguments after "--" are left untouched.
func StripSelfUpdateFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i, arg := range args {
		if arg == "--" {
			return append(out, args[i:]...)
		}
		if arg == selfUpdateFlag || strings.HasPrefix(arg, selfUpdateFlag+"=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}
