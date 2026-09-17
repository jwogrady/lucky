package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jwogrady/lucky/internal/cli"
	"github.com/jwogrady/lucky/internal/opclient"
)

var version = "dev"

func main() {
	app := cli.App{
		Out: os.Stdout,
		Err: os.Stderr,
		In:  os.Stdin,
		NewClient: func(ctx context.Context, cfg cli.Config) (cli.Client, error) {
			return opclient.New(ctx, opclient.Config{
				Account: cfg.Account,
				Version: version,
			})
		},
	}
	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "lucky: %s\n", err)
		os.Exit(1)
	}
}
