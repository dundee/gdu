package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	sf := &scanFlags{}

	var rootCmd = &cobra.Command{
		Use:   "gdu [directory_to_scan]",
		Short: "Pretty fast disk usage analyzer written in Go",
		Long: `Pretty fast disk usage analyzer written in Go.

Gdu is intended primarily for SSD disks where it can fully utilize parallel processing.
However HDDs work as well, but the performance gain is not so huge.
`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			istty := isatty.IsTerminal(os.Stdout.Fd())
			scan(sf, args, istty, os.Stdout)
		},
	}

	flags := rootCmd.Flags()
	flags.StringVarP(&sf.logFile, "log-file", "l", "/dev/null", "Path to a logfile")
	flags.StringSliceVarP(&sf.ignoreDirs, "ignore-dirs", "i", []string{"/proc", "/dev", "/sys", "/run"}, "Absolute paths to ignore (separated by comma)")
	flags.BoolVarP(&sf.showVersion, "version", "v", false, "Print version")
	flags.BoolVarP(&sf.noColor, "no-color", "c", false, "Do not use colorized output")
	flags.BoolVarP(&sf.nonInteractive, "non-interactive", "n", false, "Do not run in interactive mode")
	flags.BoolVarP(&sf.noProgress, "no-progress", "p", false, "Do not show progress")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
