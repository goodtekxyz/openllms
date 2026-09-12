package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Set via -ldflags at release/dist build time.
var (
	Version = "dev"
	Commit  = "unknown"
)

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version and build metadata",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("llms %s (%s) %s/%s\n", Version, Commit, runtime.GOOS, runtime.GOARCH)
		},
	}
}
