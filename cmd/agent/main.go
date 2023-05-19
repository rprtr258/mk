package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/rprtr258/log"
	"github.com/rprtr258/mk"
	"github.com/rprtr258/mk/contrib/docker"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

const _version = "v0.0.0"

func remoteRun(user, addr string, privateKey []byte, cmd string) (string, error) {
	key, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	client, err := ssh.Dial(
		"tcp",
		net.JoinHostPort(addr, "22"),
		&ssh.ClientConfig{ //nolint:exhaustruct // daaaaaa
			User:            user,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // host key ignored
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(key),
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("connect to ssh server user=%q host=%q: %w", user, addr, err)
	}

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	errCmd := session.Run(cmd)
	return b.String(), errCmd
}

func main() {
	if err := (&cli.App{ //nolint:exhaustruct // daaaaa
		Name:            "mk-agent",
		HideHelpCommand: true,
		HideHelp:        true,
		Commands: []*cli.Command{
			{
				Name: "test",
				Action: func(ctx *cli.Context) error {
					cwd, errCwd := os.Getwd()
					if errCwd != nil {
						return fmt.Errorf("get cwd: %w", errCwd)
					}

					if _, _, errBuild := mk.ExecContext(ctx.Context,
						"go", "build", filepath.Join(cwd, "cmd/agent"),
					); errBuild != nil {
						return fmt.Errorf("build agent: %w", errBuild)
					}

					privateKey, errKey := os.ReadFile("/home/rprtr258/.ssh/rus_rprtr258")
					if errKey != nil {
						return fmt.Errorf("read private key: %w", errKey)
					}

					output, errRun := remoteRun("rprtr258", "rus", privateKey, "hostname && ls -la")
					if errRun != nil {
						return errRun
					}

					log.Info(output)

					return nil
				},
			},
			{
				Name:  "version",
				Usage: "Show mk agent version",
				Action: func(*cli.Context) error {
					fmt.Println(_version)
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
