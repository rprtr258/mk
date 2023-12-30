package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/rprtr258/mk"
	"github.com/rprtr258/mk/cache"
	"github.com/rprtr258/mk/ssh"
)

const (
	_agentExecutable = "mk-agent"
)

var errRemoteAgentNotFound = errors.New("mk-agent is not installed remotely")

func getRemoteAgentHash(
	ctx context.Context,
	conn ssh.Connection,
) (string, error) {
	l := log.With().
		Str("host", conn.Host()).
		Str("user", conn.User()).
		Str("component", "getRemoteAgentHash").
		Logger()

	// TODO: behold not having sha256sum on remote
	stdout, stderr, errAgentVersion := conn.Run(ctx, "sha256sum mk-agent")
	if errAgentVersion != nil {
		if strings.Contains(string(stderr), "sha256sum: mk-agent: No such file or directory") {
			l.Info().Msg("mk-agent is not installed remotely")
			return "", errRemoteAgentNotFound
		}

		return "", fmt.Errorf("get actual mk-agent version: %w", errAgentVersion)
	}

	return string(stdout[:64]), nil
}

func BuildLocally(ctx context.Context) error {
	if _, _, errBuild := mk.ExecContext(ctx,
		"go", "build", "-o", _agentExecutable, filepath.Join("cmd", _agentExecutable, "main.go"),
	); errBuild != nil {
		return fmt.Errorf("go build %s: %w", _agentExecutable, errBuild)
	}

	if _, _, errBuild := mk.ExecContext(ctx,
		"strip", _agentExecutable,
	); errBuild != nil {
		return fmt.Errorf("strip %s: %w", _agentExecutable, errBuild)
	}

	if _, _, errBuild := mk.ExecContext(ctx,
		"upx", _agentExecutable,
		// TODO: on release
		// "upx", "--best", "--ultra-brute", executableName,
	); errBuild != nil {
		return fmt.Errorf("upx %s: %w", _agentExecutable, errBuild)
	}

	return nil
}

func getAgentBinary(ctx context.Context) (io.ReadCloser, error) {
	// TODO: uncomment
	// if errBuild := BuildLocally(ctx); errBuild != nil {
	// 	return nil, fmt.Errorf("build agent: %w", errBuild)
	// }

	agentFile, errOpen := os.Open(_agentExecutable)
	if errOpen != nil {
		return nil, fmt.Errorf("open agent binary: %w", errOpen)
	}

	return agentFile, nil
}

func remoteNeedsToBeUpdated(ctx context.Context, conn ssh.Connection) (bool, error) {
	remoteHash, errLocalHash := getRemoteAgentHash(ctx, conn)
	if errLocalHash != nil {
		if errors.Is(errLocalHash, errRemoteAgentNotFound) {
			return true, nil
		}

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

	log.Debug().
		Str("remoteHash", remoteHash).
		Str("localHash", localHash).
		Msg("comparing agent hashes")
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
