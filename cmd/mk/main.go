package main

import (
	"context"
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
	"github.com/urfave/cli/v2"

	"github.com/rprtr258/mk"
	"github.com/rprtr258/mk/agent"
	"github.com/rprtr258/mk/contrib/docker"
	"github.com/rprtr258/mk/ssh"
)

var _privateKey = readFile("/home/rprtr258/.ssh/rus_rprtr258")

func readFile(filename string) []byte {
	res, errKey := os.ReadFile(filename)
	if errKey != nil {
		log.Fatalf("read private key", log.F{"filename": filename, "err": errKey})
	}
	return res
}

// TODO: for local runs don't use agent
func listContainers(ctx context.Context, conn ssh.Connection) (map[string]docker.ContainerConfig, error) {
	return agent.AgentQuery[map[string]docker.ContainerConfig](
		ctx,
		conn,
		[]string{"docker", "container", "ls"}, // TODO: bind with agent declaration
	)
}

// TODO: for local runs don't use agent
func reconcileContainer(ctx context.Context, conn ssh.Connection, policies []docker.ContainerPolicy) error {
	return agent.AgentCommand( //nolint:wrapcheck // pohuy
		ctx,
		conn,
		[]string{"docker", "container", "reconcile"}, // TODO: bind with agent declaration
		policies,
	)
}

func main() {
	if err := (&cli.App{ //nolint:exhaustruct // daaaaaa
		Name:            "mk",
		Usage:           "project commands runner",
		HideHelp:        true,
		HideHelpCommand: true,
		Commands: []*cli.Command{
			{
				Name:  "build",
				Usage: "Compile/recompile mk binary for running tasks",
				Action: func(ctx *cli.Context) error {
					if _, _, err := mk.ExecContext(ctx.Context, "go", "build", "-o", "mk", "cmd/mk/main.go"); err != nil {
						return fmt.Errorf("build main.go: %w", err)
					}

					return nil
				},
				Subcommands: []*cli.Command{
					{
						Name:  "agent",
						Usage: "Compile mk-agent binary",
						// TODO: watch bool flag
						Action: func(ctx *cli.Context) error {
							if err := agent.BuildLocally(ctx.Context); err != nil {
								return fmt.Errorf("build agent: %w", err)
							}

							return nil
						},
					},
				},
			},
			{
				Name:  "test-agent",
				Usage: "Test agent upload",
				Action: func(ctx *cli.Context) error {
					conn, errSSH := ssh.NewConnection("rprtr258", "rus", _privateKey)
					if errSSH != nil {
						return fmt.Errorf("new ssh connection: %w", errSSH)
					}
					defer conn.Close()

					containers, errListContainers := listContainers(ctx.Context, conn)
					if errListContainers != nil {
						return fmt.Errorf("list containers: %w", errListContainers)
					}

					for id, container := range containers {
						log.Info(id)
						spew.Dump(container)
					}

					return nil
				},
			},
			{
				Name:  "test-reconcile",
				Usage: "Test container reconcile",
				Action: func(ctx *cli.Context) error {
					conn, errSSH := ssh.NewConnection("rprtr258", "rus", _privateKey)
					if errSSH != nil {
						return fmt.Errorf("new ssh connection: %w", errSSH)
					}
					defer conn.Close()

					policies := []docker.ContainerPolicy{
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
					}

					if errReconcile := reconcileContainer(ctx.Context, conn, policies); errReconcile != nil {
						return fmt.Errorf("reconcile: %w", errReconcile)
					}

					return nil
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}