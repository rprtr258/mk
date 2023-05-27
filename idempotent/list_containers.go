package idempotent

import (
	"context"

	"github.com/rprtr258/mk/contrib/docker"
)

// TODO: for local runs don't use agent
func ListContainers(ctx context.Context, conn SSHConnection) (map[string]docker.ContainerConfig, error) {
	return agentCall[map[string]docker.ContainerConfig](
		ctx,
		conn,
		[]string{"docker", "containers"},
	)
}
