package mk

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"

	"github.com/rprtr258/fun"
)

func run(env map[string]string, stdout, stderr io.Writer, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Env = append(os.Environ(), fun.ToSlice(env, func(k, v string) string {
		return k + "=" + v
	})...)
	c.Stderr = stderr
	c.Stdout = stdout
	c.Stdin = os.Stdin

	fmt.Printf("executing %q %v...\n", cmd, args)

	if err := c.Run(); err != nil {
		return fmt.Errorf("cmd %q %v: %w", cmd, args, err)
	}

	return nil
}

func ShellCmd(cmd string, args ...string) (stdout string, stderr string, err error) {
	absoluteCmd, err := exec.LookPath(cmd)
	if err != nil {
		return "", "", fmt.Errorf("not found %q: %w", cmd, err)
	}

	stdoutB := bytes.Buffer{}
	stderrB := bytes.Buffer{}
	if err := run(nil, &stdoutB, &stderrB, absoluteCmd, args...); err != nil {
		return "", "", fmt.Errorf(
			"command failed %q %v stdout=%q stderr=%q: %w",
			cmd,
			args,
			stdoutB.String(),
			stderrB.String(),
			err,
		)
	}

	return stdoutB.String(), stderrB.String(), nil
}

func ShellAlias(cmd string, args ...string) func(...string) (stdout string, stderr string, err error) {
	return func(nargs ...string) (string, string, error) {
		return ShellCmd(cmd, append(args, nargs...)...)
	}
}

func ShellScript(script string) (stdout string, stderr string, err error) {
	return ShellCmd("/bin/sh", "-c", script)
}

func MkDir(dir string, perms fs.FileMode) error {
	if err := os.MkdirAll(dir, perms); err != nil {
		return fmt.Errorf("mkdir %q with perms=%v: %w", dir, perms, err)
	}

	return nil
}
