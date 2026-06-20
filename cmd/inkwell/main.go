package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/grantlucas/inkwell/internal/buildinfo"
	"github.com/grantlucas/inkwell/internal/cli"
	inkwell "github.com/grantlucas/inkwell/internal/inkwell"
	"github.com/grantlucas/inkwell/internal/selfupdate"
)

// repoSlug is the GitHub owner/repo the self-update flow pulls
// releases from. Captured here rather than in selfupdate so the
// updater package stays generic.
const repoSlug = "grantlucas/inkwell"

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Options{
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		RunApp:     runApp,
		SelfUpdate: selfUpdate,
		Version:    func() error { return cli.PrintVersion(os.Stdout) },
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

// selfUpdate is the entrypoint for `inkwell self-update`. Wires the
// orchestrator to the production implementations of the three
// injected dependencies (GitHub client, downloader, replacer) and
// passes the parsed args through.
func selfUpdate(args []string) error {
	info := buildinfo.Get()
	gh := selfupdate.NewGitHubClient(repoSlug)
	dl := selfupdate.NewDownloader(http.DefaultClient)
	rp := selfupdate.NewReplacer()
	ew := selfupdate.NewExampleWriter()

	u := &selfupdate.SelfUpdater{
		CurrentVer:         info.Version,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		GOARM:              info.GOARM,
		FetchLatest:        gh.LatestRelease,
		FetchAsset:         dl.FetchVerifyExtract,
		ReplaceBinary:      rp.Replace,
		WriteExampleConfig: ew.Write,
	}
	return u.Run(args, os.Stdout)
}
