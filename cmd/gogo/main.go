package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/daFish/gogo-meta/internal/cli"
	"github.com/daFish/gogo-meta/internal/output"
	"github.com/daFish/gogo-meta/internal/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	rootCmd := cli.NewRootCommand(version.Info())
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		stop()
		for _, line := range strings.Split(err.Error(), "\n") {
			output.Error(line)
		}
		os.Exit(1)
	}
	stop()
}
