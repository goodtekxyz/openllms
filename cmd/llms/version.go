package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"

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
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Printf("llms %s (%s) %s/%s\n", Version, Commit, runtime.GOOS, runtime.GOARCH)
			if _, ok := parseDescribe(Version); !ok {
				return
			}
			base := apiBase()
			latest := latestVersion(cmd.Context(), base, time.Now(), true)
			switch {
			case latest == "":
				fmt.Println("latest: unknown (gateway dist unreachable)")
			case isNewerVersion(Version, latest):
				fmt.Print(strings.TrimPrefix(updateNotice(Version, latest, base), "\n"))
			default:
				fmt.Printf("latest: %s (up to date)\n", latest)
			}
		},
	}
}
