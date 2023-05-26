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

type ContainerState int

const (
	ContainerStateCreated ContainerState = iota
	ContainerStateRunning
	ContainerStatePaused
	ContainerStateRestarting
	ContainerStateRemoving
	ContainerStateExited
	ContainerStateDead
)

type ContainerConfig struct {
	Name     string
	Hostname string
	Image    string
	Networks []struct {
		Name string
	}
	Volumes       []string
	RestartPolicy string
	State         ContainerState
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

		var state ContainerState
		switch details.State.Status {
		case "running":
			state = ContainerStateRunning
		case "paused":
			state = ContainerStatePaused
		case "restarting":
			state = ContainerStateRestarting
		case "removing":
			state = ContainerStateRemoving
		case "exited":
			state = ContainerStateExited
		case "dead":
			state = ContainerStateDead
		default:
			return nil, fmt.Errorf("unknown container state: %s", details.State.Status)
		}

		res[container.ID] = ContainerConfig{
			Name:          details.Name,
			Hostname:      details.Config.Hostname,
			Image:         details.Image, // TODO: image hash
			Networks:      nil,           // TODO: fill
			Volumes:       fun.Keys(details.Config.Volumes),
			RestartPolicy: details.HostConfig.RestartPolicy.Name,
			State:         state,
		}
	}

	return res, nil
}

type ContainerDesiredState int

const (
	ContainerDesiredStateStarted ContainerDesiredState = iota // container must exist and be running
	ContainerDesiredStateAbsent                               // container must be stopped and removed
	ContainerDesiredStatePresent                              // container must exist
	ContainerDesiredStateStopped                              // container must exist and be stopped
)

type ContainerPolicy struct {
	Name     string
	Hostname fun.Option[string]
	Image    string
	Networks []struct {
		Name string
	}
	Volumes       []string
	RestartPolicy fun.Option[string]
	State         ContainerDesiredState
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

func Reconciliate(container ContainerConfig, policy ContainerPolicy) error {
	if NeedsRecreate(container, policy) {
		return nil // TODO: recreate
	}

	switch policy.State {
	case ContainerDesiredStateStarted:
		switch container.State {
		case ContainerStateRunning, ContainerStateRestarting:
			return nil
		case ContainerStateCreated, ContainerStatePaused, ContainerStateExited:
			return nil // TODO: run
		case ContainerStateRemoving:
			return nil // TODO: create, then run
		case ContainerStateDead:
			return fmt.Errorf("don't know how to start from dead state")
		default:
			return fmt.Errorf("don't know how to start from state %v", container.State)
		}
	case ContainerDesiredStateAbsent:
		switch container.State {
		case ContainerStateRemoving:
			return nil
		case ContainerStateCreated, ContainerStatePaused, ContainerStateExited:
			return nil // TODO: remove
		case ContainerStateRunning, ContainerStateRestarting:
			return nil // TODO: stop, then remove
		case ContainerStateDead:
			return fmt.Errorf("don't know how to remove from dead state")
		default:
			return fmt.Errorf("don't know how to remove from state %v", container.State)
		}
	case ContainerDesiredStatePresent:
		switch container.State {
		case ContainerStateCreated, ContainerStateRunning,
			ContainerStatePaused, ContainerStateRestarting, ContainerStateExited:
			return nil
		case ContainerStateDead:
			return fmt.Errorf("don't know how to become present from dead state")
		case ContainerStateRemoving:
			return nil // TODO: start
		default:
			return fmt.Errorf("don't know how to become present from state %v", container.State)
		}
	case ContainerDesiredStateStopped:
		switch container.State {
		case ContainerStateCreated, ContainerStatePaused, ContainerStateRemoving, ContainerStateExited:
			return nil
		case ContainerStateRunning, ContainerStateRestarting:
			return nil // TODO: stop
		case ContainerStateDead:
			return fmt.Errorf("don't know how to stop from dead state")
		default:
			return fmt.Errorf("don't know how to stop from state %v", container.State)
		}
	default:
		return fmt.Errorf("unknown container desired state: %v", policy.State)
	}
}
