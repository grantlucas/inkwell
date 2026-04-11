package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	inkwell "github.com/grantlucas/inkwell/internal/inkwell"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "inkwell: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath := "inkwell.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	f, err := os.Open(cfgPath)
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
