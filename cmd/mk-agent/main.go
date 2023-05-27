package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rprtr258/fun"
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
				Usage: "Show mk-agent version",
				Action: func(*cli.Context) error {
					fmt.Printf(`"%s"`, _version)
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
									client, err := docker.NewClient()
									if err != nil {
										return fmt.Errorf("new client: %w", err)
									}

									containers, err := client.ContainersList(ctx.Context)
									if err != nil {
										return fmt.Errorf("get containers: %w", err)
									}

									matching := docker.MatchPolicies(containers, []docker.ContainerPolicy{
										// TODO: get from args
										{
											Name:          "test-container",
											Hostname:      fun.Option[string]{},
											Image:         "alpine:latest",
											Networks:      nil,
											Volumes:       nil,
											RestartPolicy: fun.Option[docker.RestartPolicy]{},
											State:         docker.ContainerDesiredStateAbsent,
											Cmd:           fun.Valid([]string{"sleep", "infinity"}),
										},
									})

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
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
