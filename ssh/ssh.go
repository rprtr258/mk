package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/pkg/sftp"
	"github.com/rprtr258/log"
	"go.uber.org/multierr"
	"golang.org/x/crypto/ssh"
)

type Connection struct {
	client *ssh.Client
	sftp   *sftp.Client

	user string
	host string

	l log.Logger
}

func NewConnection(user, host string, privateKey []byte) (Connection, error) {
	key, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return Connection{}, fmt.Errorf("parse private key: %w", err)
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
		return Connection{}, fmt.Errorf("connect to ssh server user=%q host=%q: %w", user, host, err)
	}

	sftp, err := sftp.NewClient(client)
	if err != nil {
		return Connection{}, fmt.Errorf("new sftp client: %w", err)
	}

	return Connection{
		client: client,
		sftp:   sftp,
		user:   user,
		host:   host,
		l:      log.Tag("ssh").With(log.F{"user": user, "host": host}),
	}, nil
}

func (conn Connection) Close() error {
	var merr error
	if errSFTP := conn.sftp.Close(); errSFTP != nil {
		multierr.AppendInto(&merr, fmt.Errorf("close sftp client: %w", errSFTP))
	}
	if errSSH := conn.client.Close(); errSSH != nil {
		multierr.AppendInto(&merr, fmt.Errorf("close ssh client: %w", errSSH))
	}
	return merr
}

func (conn Connection) Run(cmd string) ( //nolint:nonamedreturns // too many returns
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

func (conn Connection) Upload(r io.Reader, remotePath string, mode os.FileMode) error {
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

func (conn Connection) Host() string {
	return conn.host
}

func (conn Connection) User() string {
	return conn.user
}
