package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rprtr258/log"
	"github.com/urfave/cli/v2"

	"github.com/rprtr258/mk/contrib/docker"
)

var _subcommandDocker = &cli.Command{ //nolint:exhaustruct // pohuy
	Name:            "docker",
	Usage:           "Get info about docker resources",
	HideHelpCommand: true,
	HideHelp:        true,
	Subcommands: []*cli.Command{
		{
			Name:            "container",
			Usage:           "Get info about docker containers",
			HideHelpCommand: true,
			HideHelp:        true,
			Subcommands: []*cli.Command{
				{
					Name:            "ls",
					Usage:           "List containers",
					HideHelpCommand: true,
					HideHelp:        true,
					Action: func(ctx *cli.Context) error {
						client, err := docker.NewClient()
						if err != nil {
							return fmt.Errorf("new client: %w", err)
						}

						containers, err := client.ContainersList(ctx.Context)
						if err != nil {
							return fmt.Errorf("get containers: %w", err)
						}

						resp, errMarshal := json.Marshal(containers)
						if errMarshal != nil {
							return fmt.Errorf("json marshal containers list: %w", errMarshal)
						}

						fmt.Print(string(resp))

						return nil
					},
				},
				{
					// TODO: change to multiple containers
					Name:            "reconcile",
					Usage:           "Reconcile container",
					HideHelpCommand: true,
					HideHelp:        true,
					Action: func(ctx *cli.Context) error {
						if ctx.Args().Len() != 1 {
							return fmt.Errorf("expected single argument, but got %d", ctx.Args().Len())
						}

						var policies []docker.ContainerPolicy
						arg := ctx.Args().First()
						if errUnmarshal := json.Unmarshal([]byte(arg), &policies); errUnmarshal != nil {
							return fmt.Errorf("unmarshal policies, arg=%q: %w", arg, errUnmarshal)
						}

						client, err := docker.NewClient()
						if err != nil {
							return fmt.Errorf("new client: %w", err)
						}

						containers, err := client.ContainersList(ctx.Context)
						if err != nil {
							return fmt.Errorf("get containers: %w", err)
						}

						matching := docker.MatchPolicies(containers, policies)

						for _, match := range matching {
							if errReconcile := docker.Reconcile(ctx.Context, client, match.Policy, match.Container); errReconcile != nil {
								// TODO: print errors as json to parse on client
								return fmt.Errorf("reconcile, policy=%+v: %w", match.Policy, errReconcile)
							}
						}

						return nil
					},
				},
			},
		},
	},
}

func main() {
	if err := (&cli.App{ //nolint:exhaustruct // daaaaa
		Name:            "mk-agent",
		HideHelpCommand: true,
		HideHelp:        true,
		Commands: []*cli.Command{
			_subcommandDocker,
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
