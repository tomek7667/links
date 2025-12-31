package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/tomek7667/links/internal/http"
	"github.com/tomek7667/links/internal/json"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "linksserver",
		Description: "simple http server displaying links to your services with local json database",
		Usage:       "serve or manage the linksserver binary (use subcommands)",
		Version:     appVersion(),
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				EnvVars: []string{"PORT"},
				Value:   80,
			},
		},
		Commands: []*cli.Command{
			cmdUpdate(),
			cmdCompleteUpdate(),
		},
		CommandNotFound: func(c *cli.Context, command string) {
			fmt.Fprintf(os.Stderr, "unknown command %q\n\n", command)
			cli.ShowAppHelpAndExit(c, 1)
		},
		Action: func(c *cli.Context) error {
			db, err := json.New()
			if err != nil {
				return fmt.Errorf("failed to create json database: %w", err)
			}
			port := c.Int("port")
			server := http.New(port, db)
			return server.Serve()
		},
		BashComplete: cli.ShowCompletions,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func appVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi == nil {
		return "unknown"
	}

	version := bi.Main.Version
	var rev string
	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if version != "" && version != "(devel)" {
		return version
	}
	if rev != "" {
		if modified {
			return rev + " (modified)"
		}
		return rev
	}
	if version != "" {
		return version
	}
	return "unknown"
}
