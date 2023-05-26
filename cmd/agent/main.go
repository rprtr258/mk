package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/rprtr258/log"
	"github.com/urfave/cli/v2"

	"github.com/rprtr258/mk/contrib/docker"
)

const _version = "v0.0.0" // TODO: change to datetime

func main() {
	if err := (&cli.App{ //nolint:exhaustruct // daaaaa
		Name:            "mk-agent",
		HideHelpCommand: true,
		HideHelp:        true,
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Show mk agent version",
				Action: func(*cli.Context) error {
					fmt.Print(_version)
					return nil
				},
			},
			{
				Name:            "docker",
				Usage:           "Get info about docker resources",
				HideHelpCommand: true,
				HideHelp:        true,
				Subcommands: []*cli.Command{
					{
						Name:  "containers",
						Usage: "Get info about docker containers",
						Action: func(ctx *cli.Context) error {
							client, err := docker.NewClient()
							if err != nil {
								return fmt.Errorf("new client: %w", err)
							}

							containers, err := client.Containers(ctx.Context)
							if err != nil {
								return fmt.Errorf("get containers: %w", err)
							}

							spew.Dump(containers)

							return nil
						},
					},
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
