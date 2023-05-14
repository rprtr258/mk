package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	dockerClient "github.com/docker/docker/client"
	"github.com/rprtr258/fun"
)

type Client struct {
	client *dockerClient.Client
}

func NewClient() (Client, error) {
	client, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv)
	if err != nil {
		return Client{}, fmt.Errorf("new docker client: %w", err)
	}

	return Client{client}, nil
}

type ContainerConfig struct {
	Name     string
	Hostname string
	Image    string
	Networks []struct {
		Name string
	}
	Volumes       []string
	RestartPolicy string
}

func (c Client) Containers(ctx context.Context) (map[string]ContainerConfig, error) {
	containers, err := c.client.ContainerList(ctx, types.ContainerListOptions{}) //nolint:exhaustruct // no options
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	res := make(map[string]ContainerConfig, len(containers))

	for _, container := range containers {
		details, errInspect := c.client.ContainerInspect(ctx, container.ID)
		if errInspect != nil {
			return nil, fmt.Errorf("inspect container %s: %w", containers[0].ID, errInspect)
		}

		res[container.ID] = ContainerConfig{
			Name:          details.Name,
			Hostname:      details.Config.Hostname,
			Image:         details.Image, // TODO: image hash
			Networks:      nil,           // TODO: fill
			Volumes:       fun.Keys(details.Config.Volumes),
			RestartPolicy: details.HostConfig.RestartPolicy.Name,
		}
	}

	return res, nil
}

type ContainerPolicy struct {
	Name     string
	Hostname fun.Option[string]
	Image    string
	Networks []struct {
		Name string
	}
	Volumes       []string
	RestartPolicy fun.Option[string]
}

func compareLists[T comparable](xs, ys []T) bool {
	if len(xs) != len(ys) {
		return false
	}

	aSet := fun.ToMap(xs, func(elem T) (T, struct{}) {
		return elem, struct{}{}
	})

	return fun.All(ys, func(s T) bool {
		_, ok := aSet[s]
		return ok
	})
}

func NeedsRecreate(container ContainerConfig, policy ContainerPolicy) bool {
	return container.Name != policy.Name ||
		(policy.Hostname.Valid() && container.Hostname != policy.Hostname.Unwrap()) ||
		container.Image != policy.Image ||
		!compareLists(container.Networks, policy.Networks) ||
		!compareLists(container.Volumes, policy.Volumes) ||
		(policy.RestartPolicy.Valid() && container.RestartPolicy != policy.RestartPolicy.Unwrap())
}
