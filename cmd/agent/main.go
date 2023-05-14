package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/rprtr258/log"
	"github.com/rprtr258/mk/contrib/docker"
	"github.com/urfave/cli/v2"
)

func main() {
	if err := (&cli.App{ //nolint:exhaustruct // daaaaa
		Name: "mk-agent",
		Commands: []*cli.Command{
			{
				Name:  "docker",
				Usage: "Get info about docker resources",
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
