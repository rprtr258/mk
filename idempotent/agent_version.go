package idempotent

import (
	"context"
)

type agentVersion struct {
	User       string
	Host       string
	PrivateKey []byte
}

type AgentVersionOptions struct {
	User       string
	Host       string
	PrivateKey []byte
}

func NewAgentVersion(opts AgentVersionOptions) Action[string] {
	return &agentVersion{
		User:       opts.User,
		Host:       opts.Host,
		PrivateKey: opts.PrivateKey,
	}
}

func (a *agentVersion) IsCompleted() (bool, error) {
	return false, nil
}

func (a *agentVersion) Perform(ctx context.Context) (string, error) {
	return agentCall[string](ctx, a.User, a.Host, a.PrivateKey, []string{"version"})
}
