package idempotent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/sftp"
	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
	"github.com/rprtr258/mk/cache"
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

	l log.Logger
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
		l:      log.Tag("ssh").With(log.F{"user": user, "host": host}),
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
	// TODO: use context like here https://github.com/umputun/spot/blob/master/pkg/executor/remote.go#L239
	sess, err := conn.client.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	var outB, errB bytes.Buffer
	sess.Stdout, sess.Stderr = &outB, &errB
	conn.l.Debugf("executing command remotely", log.F{"command": cmd})
	errCmd := sess.Run(cmd)
	// TODO: multiwrite to stdout
	conn.l.Debugf("command finished", log.F{
		"command": cmd,
		"stdout":  outB.String(),
		"stderr":  errB.String(),
	})
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

	conn.l.Debugf("uploading file", log.F{"remotePath": remotePath})
	if _, errUpload := dstFile.ReadFrom(r); errUpload != nil {
		return fmt.Errorf("write to remote file %q: %w", remotePath, errUpload)
	}

	return nil
}

func getRemoteAgentHash(
	_ context.Context,
	conn SSHConnection,
) (string, error) {
	l := conn.l.Tag("checkAgentInstalled")

	// TODO: behold not having sha256sum on remote
	stdout, stderr, errAgentVersion := conn.Run("sha256sum mk-agent")
	if errAgentVersion != nil {
		if strings.Contains(string(stderr), "sha256sum: mk-agent: No such file or directory") {
			l.Info("mk-agent is not installed remotely")
			return "", nil // TODO: not found error
		}

		return "", fmt.Errorf("get actual mk-agent version: %w", errAgentVersion)
	}

	return string(stdout[:64]), nil
}

func getAgentBinary(ctx context.Context) (io.ReadCloser, error) {
	cwd, errCwd := os.Getwd()
	if errCwd != nil {
		return nil, fmt.Errorf("get cwd: %w", errCwd)
	}

	if _, _, errBuild := mk.ExecContext(ctx,
		"go", "build", filepath.Join(cwd, "cmd", _agentExecutable),
	); errBuild != nil {
		return nil, fmt.Errorf("build agent: %w", errBuild)
	}

	agentBinaryPath := filepath.Join(cwd, _agentExecutable)

	if _, _, errBuild := mk.ExecContext(ctx,
		"strip", agentBinaryPath,
	); errBuild != nil {
		return nil, fmt.Errorf("strip agent binary: %w", errBuild)
	}

	if _, _, errBuild := mk.ExecContext(ctx,
		"upx", agentBinaryPath,
	); errBuild != nil {
		return nil, fmt.Errorf("upx agent binary: %w", errBuild)
	}

	agentFile, errOpen := os.Open(agentBinaryPath)
	if errOpen != nil {
		return nil, fmt.Errorf("open agent binary: %w", errOpen)
	}

	return agentFile, nil
}

func remoteNeedsToBeUpdated(ctx context.Context, conn SSHConnection) (bool, error) {
	remoteHash, errLocalHash := getRemoteAgentHash(ctx, conn)
	if errLocalHash != nil {
		return false, errLocalHash
	}

	if remoteHash == "" {
		// not installed
		return true, nil
	}

	cwd, errCwd := os.Getwd()
	if errCwd != nil {
		return false, fmt.Errorf("get cwd: %w", errCwd)
	}

	// TODO: get remote in prod, local in dev
	localHash, errLocalHash := cache.HashFile(filepath.Join(cwd, _agentExecutable))
	if errLocalHash != nil {
		return false, fmt.Errorf("get local agent binary hash: %w", errLocalHash)
	}

	log.Debugf("comparing agent hashes", log.F{
		"remoteHash": remoteHash,
		"localHash":  localHash,
	})
	return remoteHash != localHash, nil
}

func installAgent(ctx context.Context, conn SSHConnection) error {
	// TODO: cache
	needToUpdate, errCheck := remoteNeedsToBeUpdated(ctx, conn)
	if errCheck != nil {
		return fmt.Errorf("check if remote needs to be updated: %w", errCheck)
	}

	if !needToUpdate {
		return nil
	}

	// TODO: switch between compile and download from github
	agentFile, errBuild := getAgentBinary(ctx)
	if errBuild != nil {
		return fmt.Errorf("get agent binary: %w", errBuild)
	}
	defer agentFile.Close()

	remoteAgentBinaryPath := "./" + _agentExecutable

	const agentFilePerms = 0o700
	if errUpload := conn.Upload(agentFile, remoteAgentBinaryPath, agentFilePerms); errUpload != nil {
		return fmt.Errorf("upload agent binary: %w", errUpload)
	}

	return nil
}

func AgentQuery[T any](
	ctx context.Context,
	conn SSHConnection,
	cmd []string,
) (T, error) {
	if errInstall := installAgent(ctx, conn); errInstall != nil {
		return fun.Zero[T](), errInstall
	}

	// TODO: gzip args, validate args length, chunk args
	stdout, stderr, errRun := conn.Run(strings.Join(append([]string{"./mk-agent"}, cmd...), " "))
	if errRun != nil {
		return fun.Zero[T](), fmt.Errorf("agent call, cmd=%v, stderr=%q: %w", cmd, string(stderr), errRun)
	}

	var result T
	if errUnmarshal := json.Unmarshal(stdout, &result); errUnmarshal != nil {
		return fun.Zero[T](), fmt.Errorf(
			"json unmarshal call result, cmd=%v, stdout=%q: %w",
			cmd,
			string(stdout),
			errUnmarshal,
		)
	}

	return result, nil
}

func AgentCommand[T any](
	ctx context.Context,
	conn SSHConnection,
	cmd []string,
	arg T,
) error {
	argBytes, errMarshal := json.Marshal(arg)
	if errMarshal != nil {
		return fmt.Errorf("json marshal arg=%+v: %w", arg, errMarshal)
	}

	args := append([]string{"./mk-agent"}, cmd...)
	// TODO: gzip args, validate args length, chunk args
	args = append(args, strconv.Quote(string(argBytes)))
	fullCmd := strings.Join(args, " ")

	if errInstall := installAgent(ctx, conn); errInstall != nil {
		return errInstall
	}

	stdout, stderr, errRun := conn.Run(fullCmd)
	if errRun != nil {
		return fmt.Errorf("agent call, cmd=%v, stderr=%q: %w", cmd, string(stderr), errRun)
	}

	if len(stdout) != 0 {
		log.Infof(string(stdout), log.F{"cmd": cmd})
	}

	return nil
}
