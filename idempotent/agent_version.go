package idempotent

import (
	"context"
)

type agentVersion struct {
	conn SSHConnection
}

type AgentVersionOptions struct {
	Conn SSHConnection
}

func NewAgentVersion(opts AgentVersionOptions) Action[string] {
	return &agentVersion{
		conn: opts.Conn,
	}
}

func (a *agentVersion) IsCompleted() (bool, error) {
	return false, nil
}

func (a *agentVersion) Perform(ctx context.Context) (string, error) {
	return AgentQuery[string](ctx, a.conn, []string{"version"})
}
