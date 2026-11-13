//go:build !windows

package remove

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/pkg/fs"
)

const (
	// shellBin is the shell used to evaluate the trash command. A POSIX shell is
	// used deliberately instead of $SHELL, because the command is assembled with
	// "$@" and not every interactive shell handles it the same way.
	shellBin = "/bin/sh"

	// trashPathEnvVar holds the absolute path of the item handed to the command.
	trashPathEnvVar = "GDU_TRASH_PATH"

	// maxStderrLen caps how much of the command output ends up in the error
	// message, so a chatty command cannot flood the error modal.
	maxStderrLen = 1024
)

// itemPathPlaceholder matches the shell parameters expanding to the item path.
// A command using any of them decides itself where the path goes, so gdu does
// not append it.
var itemPathPlaceholder = regexp.MustCompile(`\$(?:[1@*]|\{1\})`)

type trashCommandOSOps struct {
	abs     func(string) (string, error)
	lstat   func(string) (os.FileInfo, error)
	environ func() []string
	run     func(argv0 string, argv, envv []string, stderr io.Writer) error
}

var trashCommandOS = trashCommandOSOps{
	abs:     filepath.Abs,
	lstat:   os.Lstat,
	environ: os.Environ,
	run:     runTrashCommand,
}

// TrashCommand returns a remove function which hands the item over to an
// external command instead of moving it into the built-in XDG trash.
//
// The command is evaluated by a POSIX shell with the absolute path of the item
// passed as a positional parameter, so shell features such as variable
// expansion work while item names can never be interpreted as shell syntax.
// The path is appended to the command unless the command refers to it itself
// via $1, $@ or $*, which commands taking their destination last need. The
// GDU_TRASH_PATH environment variable is set to the same absolute path.
//
// The item is removed from the in-memory tree only once the command succeeded
// and the path is really gone; a command which leaves the item in place (an
// interactive command answered with "no", for example) is not an error and
// leaves the tree untouched.
func TrashCommand(command string) func(dir, item fs.Item) error {
	return func(dir, item fs.Item) error {
		absPath, err := trashCommandOS.abs(item.GetPath())
		if err != nil {
			return err
		}

		var stderr bytes.Buffer
		argv := []string{"-c", shellScript(command), "gdu", absPath}
		envv := append(trashCommandOS.environ(), trashPathEnvVar+"="+absPath)

		if err := trashCommandOS.run(shellBin, argv, envv, &stderr); err != nil {
			return commandError(command, err, stderr.String())
		}

		if _, err := trashCommandOS.lstat(absPath); err == nil {
			// The command reported success but the item is still there. Keep the
			// tree in sync with the filesystem and leave the item listed.
			return nil
		}

		dir.RemoveFile(item)
		return nil
	}
}

// shellScript turns the configured command into the script handed to the
// shell. The item path is always available as $1; it is appended to the command
// only when the command does not place it somewhere itself.
func shellScript(command string) string {
	if itemPathPlaceholder.MatchString(command) {
		return command
	}
	return command + ` "$@"`
}

// runTrashCommand runs the trash command without attaching the terminal. The
// command must not be interactive, because gdu keeps rendering the UI (and may
// even run the command from a background worker) while it executes.
func runTrashCommand(argv0 string, argv, envv []string, stderr io.Writer) error {
	log.Printf("Running trash command: %s %v", argv0, argv)

	cmd := exec.Command(argv0, argv...) //nolint:gosec // Running a user supplied command is the point.
	cmd.Env = envv
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	return cmd.Run()
}

// commandError describes a failed trash command, including whatever the command
// wrote to its standard error output.
func commandError(command string, err error, stderr string) error {
	msg := strings.TrimSpace(stderr)
	if len(msg) > maxStderrLen {
		msg = msg[:maxStderrLen] + "..."
	}
	if msg == "" {
		return fmt.Errorf("running trash command %q: %w", command, err)
	}
	return fmt.Errorf("running trash command %q: %w: %s", command, err, msg)
}
