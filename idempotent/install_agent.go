package idempotent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"github.com/rprtr258/log"
	"github.com/rprtr258/mk"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
)

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

type installAgentInspection struct {
	agentVersion string // TODO: change to time.Time
}

// TODO: steal config, semantics from ansible
type installAgent struct {
	User       string
	Host       string
	PrivateKey []byte
	Version    string // TODO: remove?

	inspection *installAgentInspection
}

type InstallAgentOptions struct {
	User       string
	Host       string
	PrivateKey []byte
	Version    string
}

func NewInstallAgent(opts InstallAgentOptions) Action[Sentinel] {
	return &installAgent{
		User:       opts.User,
		Host:       opts.Host,
		PrivateKey: opts.PrivateKey,
		Version:    opts.Version,
		inspection: nil,
	}
}

func (a *installAgent) inspect() error {
	if a.inspection != nil {
		return nil
	}

	conn, errSSH := NewSSHConnection(a.User, a.Host, a.PrivateKey)
	if errSSH != nil {
		return fmt.Errorf("new ssh connection: %w", errSSH)
	}
	defer conn.Close()

	stdout, stderr, errAgentVersion := conn.Run("./agent version")
	if errAgentVersion != nil {
		// TODO: if not found, just return false
		return fmt.Errorf(
			"get remote mk-agent version, stderr=%q: %w",
			stderr,
			errAgentVersion,
		)
	}

	log.Infof("remote mk-agent", log.F{
		"user":             a.User,
		"host":             a.Host,
		"version-actual":   string(stdout),
		"version-expected": a.Version,
	})

	a.inspection = &installAgentInspection{
		agentVersion: string(stdout),
	}

	return nil
}

func (a *installAgent) IsCompleted() (bool, error) {
	if err := a.inspect(); err != nil {
		return false, err
	}

	return a.inspection.agentVersion == a.Version, nil
}

func (a *installAgent) perform(ctx context.Context) error {
	isCompleted, err := a.IsCompleted()
	if err != nil {
		return err
	}

	if isCompleted {
		return nil
	}

	cwd, errCwd := os.Getwd()
	if errCwd != nil {
		return fmt.Errorf("get cwd: %w", errCwd)
	}

	if _, _, errBuild := mk.ExecContext(ctx,
		"go", "build", filepath.Join(cwd, "cmd/agent"),
	); errBuild != nil {
		return fmt.Errorf("build agent: %w", errBuild)
	}

	agentBinaryPath := filepath.Join(cwd, "agent")

	if _, _, errBuild := mk.ExecContext(ctx,
		"strip", agentBinaryPath,
	); errBuild != nil {
		return fmt.Errorf("strip agent binary: %w", errBuild)
	}

	if _, _, errBuild := mk.ExecContext(ctx,
		"upx", agentBinaryPath,
	); errBuild != nil {
		return fmt.Errorf("upx agent binary: %w", errBuild)
	}

	conn, errSSH := NewSSHConnection(a.User, a.Host, a.PrivateKey)
	if errSSH != nil {
		return errSSH
	}
	defer conn.Close()

	agentFile, errOpen := os.Open(filepath.Join(cwd, "agent"))
	if errOpen != nil {
		return fmt.Errorf("open agent binary: %w", errOpen)
	}
	defer agentFile.Close()

	remoteAgentBinaryPath := "./agent" // filepath.Join(".", "agent")

	const agentFilePerms = 0o700
	if errUpload := conn.Upload(agentFile, remoteAgentBinaryPath, agentFilePerms); errUpload != nil {
		return fmt.Errorf("upload agent binary: %w", errUpload)
	}

	stdout, stderr, errRun := conn.Run(strings.Join([]string{
		remoteAgentBinaryPath,
		"version",
	}, " "))
	log.Info(string(stdout))
	if len(stderr) != 0 {
		log.Info("stderr:")
		log.Info(string(stderr))
	}
	if errRun != nil {
		return errRun
	}

	return nil
}

func (a *installAgent) Perform(ctx context.Context) (Sentinel, error) {
	return Sentinel{}, a.perform(ctx)
}
