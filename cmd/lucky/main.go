package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jwogrady/lucky"
	"github.com/jwogrady/lucky/internal/cli"
	"github.com/jwogrady/lucky/internal/config"
)

var version = "dev"

func main() {
	cfg := config.New()
	app := cli.App{
		Out: os.Stdout,
		Err: os.Stderr,
		In:  os.Stdin,
		NewClient: func(ctx context.Context, c cli.Config) (cli.Client, error) {
			account := c.Account
			if account == "" {
				account = cfg.Account()
			}
			return lucky.New(ctx, lucky.Options{
				Account: account,
				Version: version,
				Backend: lucky.Backend(cfg.Backend()),
				OpBin:   cfg.OpBin(),
			})
		},
		Config: cfg,
	}
	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		// A child's exit status is its own to report; Lucky only mirrors it.
		var exit *cli.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.Code)
		}
		fmt.Fprintf(os.Stderr, "lucky: %s\n", err)
		os.Exit(1)
	}
}
