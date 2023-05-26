package idempotent

import (
	"context"

	"github.com/rprtr258/mk/contrib/docker"
)

type listContainers struct {
	User       string
	Host       string
	PrivateKey []byte
}

type ListContainersOptions struct {
	User       string
	Host       string
	PrivateKey []byte
}

func NewListContainers(opts ListContainersOptions) Action[map[string]docker.ContainerConfig] {
	return &listContainers{
		User:       opts.User,
		Host:       opts.Host,
		PrivateKey: opts.PrivateKey,
	}
}

func (a *listContainers) IsCompleted() (bool, error) {
	return false, nil
}

func (a *listContainers) Perform(ctx context.Context) (map[string]docker.ContainerConfig, error) {
	return agentCall[map[string]docker.ContainerConfig](
		ctx,
		a.User, a.Host, a.PrivateKey,
		[]string{"docker", "containers"},
	)
}
