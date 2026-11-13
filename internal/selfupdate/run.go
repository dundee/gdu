package selfupdate

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const selfUpdateFlag = "--self-update"

// updater replaces the running binary with the latest release, as *Updater
// does.
type updater interface {
	Update(ctx context.Context) (Result, error)
}

// Outcome tells the caller what to do after Run.
type Outcome int

const (
	// OutcomeExit means there is nothing more to do.
	OutcomeExit Outcome = iota
	// OutcomeContinue means the normal run continues with the current binary.
	OutcomeContinue
	// OutcomeRestart means the new binary is run with the remaining arguments.
	OutcomeRestart
)

// Run updates gdu and decides what happens next. args are the
// command line arguments without the program name and without --self-update.
//
// When --self-update was the only argument, gdu exits afterwards. Otherwise the
// requested command still runs: with the new binary when the update succeeded,
// with the current one when there was nothing to update or the update failed.
func Run(ctx context.Context, w io.Writer, u updater, args []string) (Outcome, error) {
	onlyUpdate := len(args) == 0

	fmt.Fprintln(w, "Checking for a new version of gdu...")
	res, err := u.Update(ctx)
	switch {
	case err != nil && onlyUpdate:
		return OutcomeExit, fmt.Errorf("self-update: %w", err)
	case err != nil:
		fmt.Fprintf(w, "Self-update failed: %s\nContinuing with the current version.\n", err)
		return OutcomeContinue, nil
	case !res.Updated:
		fmt.Fprintf(w, "gdu %s is the latest version\n", res.CurrentVersion)
		if onlyUpdate {
			return OutcomeExit, nil
		}
		return OutcomeContinue, nil
	}

	fmt.Fprintf(w, "Updated gdu %s -> %s\n", res.CurrentVersion, res.LatestVersion)
	if onlyUpdate {
		return OutcomeExit, nil
	}
	return OutcomeRestart, nil
}

// StripFlag removes --self-update from args, so that the restarted
// binary does not update again. Arguments after "--" are left untouched.
func StripFlag(args []string) []string {
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
