package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/api/types"
	dockerClient "github.com/docker/docker/client"
	"github.com/rprtr258/log"
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
							client, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv)
							if err != nil {
								return fmt.Errorf("docker client: %w", err)
							}

							containers, err := client.ContainerList(ctx.Context, types.ContainerListOptions{}) //nolint:exhaustruct,lll // no options
							if err != nil {
								return fmt.Errorf("list containers: %w", err)
							}

							c0, err := client.ContainerInspect(ctx.Context, containers[0].ID)
							if err != nil {
								return fmt.Errorf("inspect container %s: %w", containers[0].ID, err)
							}

							spew.Dump(containers, c0)

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
