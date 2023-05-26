package idempotent

import (
	"context"

	"github.com/rprtr258/mk/contrib/docker"
)

type listContainers struct {
	conn SSHConnection
}

type ListContainersOptions struct {
	Conn SSHConnection
}

func NewListContainers(opts ListContainersOptions) Action[map[string]docker.ContainerConfig] {
	return &listContainers{
		conn: opts.Conn,
	}
}

func (a *listContainers) IsCompleted() (bool, error) {
	return false, nil
}

func (a *listContainers) Perform(ctx context.Context) (map[string]docker.ContainerConfig, error) {
	return agentCall[map[string]docker.ContainerConfig](
		ctx,
		a.conn,
		[]string{"docker", "containers"},
	)
}
