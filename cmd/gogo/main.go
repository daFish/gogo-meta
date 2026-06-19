package main

import (
	"context"
	"os"
	"os/signal"
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
		output.Error(err.Error())
		os.Exit(1)
	}
	stop()
}
