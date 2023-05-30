package idempotent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"

	"github.com/rprtr258/mk"
	"github.com/rprtr258/mk/cache"
	"github.com/rprtr258/mk/ssh"
)

const (
	_agentExecutable = "mk-agent"
)

func getRemoteAgentHash(
	_ context.Context,
	conn ssh.Connection,
) (string, error) {
	l := log.With(log.F{
		"host": conn.Host(),
		"user": conn.User(),
	}).Tag("getRemoteAgentHash")

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

func remoteNeedsToBeUpdated(ctx context.Context, conn ssh.Connection) (bool, error) {
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

func installAgent(ctx context.Context, conn ssh.Connection) error {
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
	conn ssh.Connection,
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
	conn ssh.Connection,
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
