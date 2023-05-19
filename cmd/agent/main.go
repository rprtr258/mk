package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/sftp"
	"github.com/rprtr258/log"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"

	"github.com/rprtr258/mk"
	"github.com/rprtr258/mk/contrib/docker"
)

const _version = "v0.0.0"

type SSHConnection struct {
	client *ssh.Client
	sftp   *sftp.Client

	user string
	host string
}

func NewSSHConnection(user, host string, privateKey []byte) (SSHConnection, error) {
	key, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return SSHConnection{}, fmt.Errorf("parse private key: %w", err)
	}

	client, err := ssh.Dial(
		"tcp",
		net.JoinHostPort(host, "22"),
		&ssh.ClientConfig{ //nolint:exhaustruct // daaaaaa
			User:            user,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // host key ignored
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(key),
			},
		},
	)
	if err != nil {
		return SSHConnection{}, fmt.Errorf("connect to ssh server user=%q host=%q: %w", user, host, err)
	}

	sftp, err := sftp.NewClient(client)
	if err != nil {
		return SSHConnection{}, fmt.Errorf("new sftp client: %w", err)
	}

	return SSHConnection{
		client: client,
		sftp:   sftp,
		user:   user,
		host:   host,
	}, nil
}

func (conn SSHConnection) Close() error {
	var merr error
	if errSFTP := conn.sftp.Close(); errSFTP != nil {
		multierr.AppendInto(&merr, fmt.Errorf("close sftp client: %w", errSFTP))
	}
	if errSSH := conn.client.Close(); errSSH != nil {
		multierr.AppendInto(&merr, fmt.Errorf("close ssh client: %w", errSSH))
	}
	return merr
}

func (conn SSHConnection) Run(cmd string) ( //nolint:nonamedreturns // too many returns
	stdout, stderr []byte,
	err error,
) {
	session, err := conn.client.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var outB, errB bytes.Buffer
	session.Stdout = &outB
	session.Stderr = &errB
	log.Debugf("executing command remotely", log.F{
		"user":    conn.user,
		"host":    conn.host,
		"command": cmd,
	})
	errCmd := session.Run(cmd)
	return outB.Bytes(), errB.Bytes(), errCmd
}

func (conn SSHConnection) Upload(r io.Reader, remotePath string, mode os.FileMode) error {
	dstFile, err := conn.sftp.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote file %q: %w", remotePath, err)
	}
	defer dstFile.Close()

	if errChmod := dstFile.Chmod(mode); errChmod != nil {
		return fmt.Errorf("chmod path=%q mode=%v: %w", remotePath, mode, errChmod)
	}

	log.Debugf("uploading file", log.F{
		"user":       conn.user,
		"host":       conn.host,
		"remotePath": remotePath,
	})
	if _, errUpload := dstFile.ReadFrom(r); errUpload != nil {
		return fmt.Errorf("write to remote file %q: %w", remotePath, errUpload)
	}

	return nil
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

					agentBinaryPath := filepath.Join(cwd, "agent")

					if _, _, errBuild := mk.ExecContext(ctx.Context,
						"strip", agentBinaryPath,
					); errBuild != nil {
						return fmt.Errorf("strip agent binary: %w", errBuild)
					}

					if _, _, errBuild := mk.ExecContext(ctx.Context,
						"upx", agentBinaryPath,
					); errBuild != nil {
						return fmt.Errorf("upx agent binary: %w", errBuild)
					}

					privateKey, errKey := os.ReadFile("/home/rprtr258/.ssh/rus_rprtr258")
					if errKey != nil {
						return fmt.Errorf("read private key: %w", errKey)
					}

					conn, errSSH := NewSSHConnection("rprtr258", "rus", privateKey)
					if errSSH != nil {
						return errSSH
					}
					defer conn.Close()

					agentFile, errOpen := os.Open(filepath.Join(cwd, "agent"))
					if errOpen != nil {
						return fmt.Errorf("open agent binary: %w", errOpen)
					}
					defer agentFile.Close()

					if errUpload := conn.Upload(agentFile, "agent", 0o700); errUpload != nil {
						return fmt.Errorf("upload agent binary: %w", errUpload)
					}

					stdout, stderr, errRun := conn.Run("./agent docker containers")
					log.Info(string(stdout))
					if len(stderr) != 0 {
						log.Info("stderr:")
						log.Info(string(stderr))
					}
					if errRun != nil {
						return errRun
					}

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
