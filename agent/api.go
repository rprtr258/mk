package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kballard/go-shellquote"
	"github.com/rprtr258/fun"
	"github.com/rs/zerolog/log"

	"github.com/rprtr258/mk/contrib/docker"
	"github.com/rprtr258/mk/ssh"
)

type Agent struct {
	conn ssh.Connection
}

func New(ctx context.Context, conn ssh.Connection) (Agent, error) {
	if errInstall := installAgent(ctx, conn); errInstall != nil {
		return Agent{}, errInstall
	}

	return Agent{
		conn: conn,
	}, nil
}

// Query - low-level interface to run agent command and get result as T
func Query[T any](
	ctx context.Context,
	agent Agent,
	cmd []string,
) (T, error) {
	if errInstall := installAgent(ctx, agent.conn); errInstall != nil {
		return fun.Zero[T](), errInstall
	}

	fullCmd := shellquote.Join(append([]string{"./mk-agent"}, cmd...)...)

	// TODO: gzip args, validate args length, chunk args
	stdout, stderr, errRun := agent.conn.Run(ctx, fullCmd)
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

// Execute - low-level interface to run agent command
func Execute[T any](
	ctx context.Context,
	agent Agent,
	cmd []string,
	arg T,
) error {
	argBytes, errMarshal := json.Marshal(arg)
	if errMarshal != nil {
		return fmt.Errorf("json marshal arg=%+v: %w", arg, errMarshal)
	}

	// TODO: gzip args, validate args length, chunk args
	fullCmd := shellquote.Join(append(append([]string{"./mk-agent"}, cmd...), string(argBytes))...)

	if errInstall := installAgent(ctx, agent.conn); errInstall != nil {
		return errInstall
	}

	stdout, stderr, errRun := agent.conn.Run(ctx, fullCmd)
	if errRun != nil {
		return fmt.Errorf("agent call, cmd=%v, stderr=%q: %w", cmd, string(stderr), errRun)
	}

	if len(stdout) != 0 {
		log.Info().Strs("cmd", cmd).Msg(string(stdout))
	}

	return nil
}

// TODO: for local runs don't use agent
func (agent Agent) ListContainers(ctx context.Context) (map[string]docker.ContainerConfig, error) {
	return Query[map[string]docker.ContainerConfig](
		ctx,
		agent,
		[]string{"docker", "container", "ls"}, // TODO: bind with agent declaration
	)
}

// TODO: for local runs don't use agent
func (agent Agent) ReconcileContainer(ctx context.Context, policies []docker.ContainerPolicy) error {
	return Execute(
		ctx,
		agent,
		[]string{"docker", "container", "reconcile"}, // TODO: bind with agent declaration
		policies,
	)
}
