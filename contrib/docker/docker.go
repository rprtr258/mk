package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerClient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
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
	ID       string
	Name     string
	Hostname string
	Image    string
	Networks []struct {
		Name string
	}
	Volumes       []string
	RestartPolicy string
	State         ContainerState
	Cmd           []string
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
			ID:       container.ID,
			Name:     details.Name,
			Hostname: details.Config.Hostname,
			// TODO: store label instaed?
			Image:         details.Image, // TODO: image hash
			Networks:      nil,           // TODO: fill
			Volumes:       fun.Keys(details.Config.Volumes),
			RestartPolicy: details.HostConfig.RestartPolicy.Name,
			State:         state,
			Cmd:           details.Config.Cmd,
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
	Cmd           fun.Option[[]string]
}

func elementsMatch[T comparable](xs, ys []T) bool {
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

func compareLists[T comparable](xs, ys []T) bool {
	if len(xs) != len(ys) {
		return false
	}

	for i, x := range xs {
		if x != ys[i] {
			return false
		}
	}

	return true
}

func needsRecreate(container ContainerConfig, policy ContainerPolicy) bool {
	return container.Name != policy.Name ||
		(policy.Hostname.Valid() && container.Hostname != policy.Hostname.Unwrap()) ||
		container.Image != policy.Image ||
		!elementsMatch(container.Networks, policy.Networks) ||
		!elementsMatch(container.Volumes, policy.Volumes) ||
		(policy.RestartPolicy.Valid() && container.RestartPolicy != policy.RestartPolicy.Unwrap()) ||
		(policy.Cmd.Valid() && !compareLists(policy.Cmd.Unwrap(), container.Cmd))
}

func (c Client) ContainerStart(ctx context.Context, container ContainerConfig) error {
	return c.client.ContainerStart( //nolint:wrapcheck // lazy
		ctx,
		container.ID,
		types.ContainerStartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		})
}

func (c Client) ContainerCreate(ctx context.Context, cfg ContainerConfig) error {
	_, err := c.client.ContainerCreate(
		ctx,
		&container.Config{ //nolint:exhaustruct // not all options are needed
			Hostname: cfg.Hostname,
			Image:    cfg.Image,
			Volumes: fun.ToMap(cfg.Volumes, func(volume string) (string, struct{}) {
				return volume, struct{}{}
			}),
			Cmd:          cfg.Cmd,
			Env:          []string{},    // TODO: fill from cfg
			ExposedPorts: nat.PortSet{}, // TODO: fill from cfg
		},
		&container.HostConfig{ //nolint:exhaustruct // not all options are needed
			RestartPolicy: container.RestartPolicy{
				Name:              cfg.RestartPolicy, // TODO: remap?
				MaximumRetryCount: 0,
			},
		},
		nil, // TODO: fill networks from cfg
		nil,
		cfg.Name,
	)
	return err //nolint:wrapcheck // lazy
}

func (c Client) ContainerRun(ctx context.Context, container ContainerConfig) error {
	if errCreate := c.ContainerCreate(ctx, container); errCreate != nil {
		return fmt.Errorf("run, create container: %w", errCreate)
	}

	if errStart := c.ContainerStart(ctx, container); errStart != nil {
		return fmt.Errorf("run, start container: %w", errStart)
	}

	return nil
}

func (c Client) ContainerStop(ctx context.Context, container ContainerConfig) error {
	return c.client.ContainerStop(ctx, container.ID, nil) //nolint:wrapcheck // lazy
}

func (c Client) ContainerRemove(ctx context.Context, container ContainerConfig) error {
	return c.client.ContainerRemove( //nolint:wrapcheck // lazy
		ctx,
		container.ID,
		types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         false,
			RemoveLinks:   false,
		})
}

// TODO: use

func (c Client) ContainerRestart(ctx context.Context, container ContainerConfig) error {
	// TODO: do in one docker client call
	if errStop := c.ContainerStop(ctx, container); errStop != nil {
		return fmt.Errorf("restart container %#v, stopping: %w", container, errStop)
	}

	if errStart := c.ContainerStart(ctx, container); errStart != nil {
		return fmt.Errorf("restart container %#v, starting: %w", container, errStart)
	}

	return nil
}

func Reconciliate(ctx context.Context, client Client, container ContainerConfig, policy ContainerPolicy) error {
	if needsRecreate(container, policy) {
		// TODO: if remove stops container, it doesn't need to be stopped beforehand
		return nil // TODO: recreate = stop, remove, run
	}

	switch policy.State {
	case ContainerDesiredStateStarted:
		switch container.State {
		case ContainerStateRunning, ContainerStateRestarting:
			return nil
		case ContainerStateCreated, ContainerStatePaused, ContainerStateExited:
			return client.ContainerStart(ctx, container)
		case ContainerStateRemoving:
			return client.ContainerRun(ctx, container)
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
			return client.ContainerRemove(ctx, container)
		case ContainerStateRunning, ContainerStateRestarting:
			return client.ContainerRemove(ctx, container)
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
			return client.ContainerCreate(ctx, container)
		default:
			return fmt.Errorf("don't know how to become present from state %v", container.State)
		}
	case ContainerDesiredStateStopped:
		switch container.State {
		case ContainerStateCreated, ContainerStatePaused, ContainerStateRemoving, ContainerStateExited:
			return nil
		case ContainerStateRunning, ContainerStateRestarting:
			return client.ContainerStop(ctx, container)
		case ContainerStateDead:
			return fmt.Errorf("don't know how to stop from dead state")
		default:
			return fmt.Errorf("don't know how to stop from state %v", container.State)
		}
	default:
		return fmt.Errorf("unknown container desired state: %v", policy.State)
	}
}
