package mk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
)

func run(ctx context.Context, env map[string]string, stdout, stderr io.Writer, cmd string, args ...string) error {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Env = append(os.Environ(), fun.ToSlice(env, func(k, v string) string {
		return k + "=" + v
	})...)
	c.Stderr = stderr
	c.Stdout = stdout
	c.Stdin = os.Stdin

	log.Debugf("executing command", log.F{"cmd": cmd, "args": args})

	if err := c.Run(); err != nil {
		return fmt.Errorf("cmd %q %v: %w", cmd, args, err)
	}

	return nil
}

func ExecContext(ctx context.Context, cmd string, args ...string) (stdout string, stderr string, err error) {
	absoluteCmd, err := exec.LookPath(cmd)
	if err != nil {
		return "", "", fmt.Errorf("not found %q: %w", cmd, err)
	}

	stdoutB := bytes.Buffer{}
	stderrB := bytes.Buffer{}
	if err := run(ctx, nil, &stdoutB, &stderrB, absoluteCmd, args...); err != nil {
		return "", "", fmt.Errorf(
			"command failed %q %v stdout=%q stderr=%q: %w",
			cmd,
			args,
			stdoutB.String(),
			stderrB.String(),
			err,
		)
	}

	return strings.TrimSpace(stdoutB.String()), stderrB.String(), nil
}

func ExecAliasContext(cmd string, args ...string) func(context.Context, ...string) (stdout string, stderr string, err error) {
	return func(ctx context.Context, nargs ...string) (string, string, error) {
		return ExecContext(ctx, cmd, append(args, nargs...)...)
	}
}

func ShellScript(ctx context.Context, script string) (stdout string, stderr string, err error) {
	return ExecContext(ctx, "/bin/sh", "-c", script)
}

func MkDir(dir string, perms fs.FileMode) error {
	if err := os.MkdirAll(dir, perms); err != nil {
		return fmt.Errorf("mkdir %q with perms=%v: %w", dir, perms, err)
	}

	return nil
}
