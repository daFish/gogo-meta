package main

import (
	"os"

	"github.com/daFish/gogo-meta/internal/cli"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/daFish/gogo-meta/internal/version"
)

func main() {
	rootCmd := cli.NewRootCommand(version.Info())
	if err := rootCmd.Execute(); err != nil {
		output.Error(err.Error())
		os.Exit(1)
	}
}
