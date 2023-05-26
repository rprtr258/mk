package idempotent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	if errInstall := installAgent(ctx, a.User, a.Host, a.PrivateKey); errInstall != nil {
		return nil, fmt.Errorf("install agent: %w", errInstall)
	}

	// TODO: reuse ssh conn
	conn, errSSH := NewSSHConnection(a.User, a.Host, a.PrivateKey)
	if errSSH != nil {
		return nil, errSSH
	}
	defer conn.Close()

	stdout, stderr, errRun := conn.Run(strings.Join([]string{
		"./mk-agent", "docker", "containers",
	}, " "))
	if errRun != nil {
		return nil, fmt.Errorf("lookup containers, stderr=%q: %w", string(stderr), errRun)
	}

	var containers map[string]docker.ContainerConfig
	if errUnmarshal := json.Unmarshal(stdout, &containers); errUnmarshal != nil {
		return nil, fmt.Errorf("json unmarshal containers list: %w", errUnmarshal)
	}

	return containers, nil
}
