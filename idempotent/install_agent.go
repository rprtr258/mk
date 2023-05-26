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
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"

	"github.com/rprtr258/mk"
)

const (
	_agentExecutable = "mk-agent"
	_agentVersion    = "v0.0.0" // TODO: make build time
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

func checkAgentInstalled(_ context.Context, user, host string, privateKey []byte) (bool, error) {
	l := log.With(log.F{
		"user": user,
		"host": host,
	}).Tag("checkAgentInstalled")

	// TODO: cache checks

	conn, errSSH := NewSSHConnection(user, host, privateKey)
	if errSSH != nil {
		return false, fmt.Errorf("new ssh connection: %w", errSSH)
	}
	defer conn.Close()

	stdout, stderr, errAgentVersion := conn.Run("./mk-agent version")
	if errAgentVersion != nil {
		if strings.Contains(string(stderr), "./mk-agent: No such file or directory") {
			l.Infof("mk-agent is not installed remotely", log.F{"version": string(stdout)})

			return false, nil
		}

		return false, fmt.Errorf(
			"get remote mk-agent version, stderr=%q: %w",
			stderr,
			errAgentVersion,
		)
	}

	l.Infof("got remote mk-agent version", log.F{"version": string(stdout)})

	// TODO: only in dev
	// return string(stdout) == _agentVersion, nil
	return false, nil
}

func installAgent(ctx context.Context, user, host string, privateKey []byte) error {
	isInstalled, err := checkAgentInstalled(ctx, user, host, privateKey)
	if err != nil {
		return err
	}

	if isInstalled {
		return nil
	}

	cwd, errCwd := os.Getwd()
	if errCwd != nil {
		return fmt.Errorf("get cwd: %w", errCwd)
	}

	if _, _, errBuild := mk.ExecContext(ctx,
		"go", "build", filepath.Join(cwd, "cmd", _agentExecutable),
	); errBuild != nil {
		return fmt.Errorf("build agent: %w", errBuild)
	}

	agentBinaryPath := filepath.Join(cwd, _agentExecutable)

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

	// TODO: reuse ssh conn
	conn, errSSH := NewSSHConnection(user, host, privateKey)
	if errSSH != nil {
		return errSSH
	}
	defer conn.Close()

	agentFile, errOpen := os.Open(filepath.Join(cwd, _agentExecutable))
	if errOpen != nil {
		return fmt.Errorf("open agent binary: %w", errOpen)
	}
	defer agentFile.Close()

	remoteAgentBinaryPath := "./" + _agentExecutable

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
