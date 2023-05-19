package main

import (
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/rprtr258/log"
	"github.com/rprtr258/mk/contrib/docker"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
)

const _version = "v0.0.0"

func remoteRun(user, addr, privateKey, cmd string) (string, error) {
	// read privateKey
	key, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return "", err
	}

	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(addr, "22"), config)
	if err != nil {
		return "", err
	}

	session, err := client.NewSession()
	if err != nil {
		return "", err
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
