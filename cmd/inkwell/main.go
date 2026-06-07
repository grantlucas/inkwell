package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/grantlucas/inkwell/internal/cli"
	inkwell "github.com/grantlucas/inkwell/internal/inkwell"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Options{
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		RunApp:       runApp,
		SelfUpdate:   selfUpdate,
		VersionShort: func() error { return cli.PrintVersionShort(os.Stdout) },
		VersionLong:  func([]string) error { return cli.PrintVersionLong(os.Stdout) },
	}))
}

// runApp loads the config at path and starts the dashboard. Signal
// handling (SIGINT/SIGTERM → graceful shutdown) is wired here so the
// router stays generic.
func runApp(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	cfg, err := inkwell.LoadConfig(f)
	if err != nil {
		return err
	}

	app, err := inkwell.NewApp(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer stop()

	return app.Run(ctx)
}

// selfUpdate is the entrypoint for `inkwell self-update`. Wired in a
// later commit; for now it returns a clear "not yet wired" error so
// the router still has a real handler to dispatch into.
func selfUpdate(args []string) error {
	_ = args
	return fmt.Errorf("self-update is not wired in this build")
}
