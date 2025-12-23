package main

import (
	"fmt"
	"log"
	"os"

	"github.com/tomek7667/links/internal/http"
	"github.com/tomek7667/links/internal/json"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "links",
		Description: "simple http server displaying links to your services with local json database",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				EnvVars: []string{"PORT"},
				Value:   80,
			},
		},
		Action: func(c *cli.Context) error {
			db, err := json.New()
			if err != nil {
				return fmt.Errorf("failed to create json database: %w", err)
			}
			port := c.Int("port")
			server := http.New(port, db)
			server.Serve()
			return nil
		},
		BashComplete: cli.ShowCompletions,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
