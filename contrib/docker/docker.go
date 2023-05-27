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

type ContainerID string

type ContainerConfig struct {
	ID       ContainerID
	Name     string
	Hostname string
	Image    string
	Networks []struct {
		Name string
	}
	Volumes       []string
	RestartPolicy RestartPolicy
	State         ContainerState
	Cmd           []string
}

func (c Client) ContainersList(ctx context.Context) (map[ContainerID]ContainerConfig, error) {
	containers, err := c.client.ContainerList(
		ctx,
		types.ContainerListOptions{ //nolint:exhaustruct // other options are not needed
			All: true,
		})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	res := make(map[ContainerID]ContainerConfig, len(containers))

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

		var restartPolicy RestartPolicy
		switch details.HostConfig.RestartPolicy.Name {
		case "no", "":
			restartPolicy = RestartPolicyNo
		case "on-failure":
			restartPolicy = RestartPolicyOnFailure
		case "unless-stopped":
			restartPolicy = RestartPolicyUnlessStopped
		case "always":
			restartPolicy = RestartPolicyAlways
		default:
			return nil, fmt.Errorf("unknown restart policy: %q", details.HostConfig.RestartPolicy.Name)
		}

		containerID := ContainerID(container.ID)
		res[containerID] = ContainerConfig{
			ID:       containerID,
			Name:     details.Name[1:], // strip leading slash
			Hostname: details.Config.Hostname,
			// TODO: store label instead?
			Image:         details.Image, // TODO: image hash
			Networks:      nil,           // TODO: fill
			Volumes:       fun.Keys(details.Config.Volumes),
			RestartPolicy: restartPolicy,
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

type RestartPolicy int

const (
	RestartPolicyNo = iota
	RestartPolicyOnFailure
	RestartPolicyUnlessStopped
	RestartPolicyAlways
)

type ContainerPolicy struct {
	Name     string
	Hostname fun.Option[string]
	Image    string
	Networks []struct {
		Name string
	}
	Volumes       []string
	RestartPolicy fun.Option[RestartPolicy]
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

func needsRecreate(
	ctx context.Context,
	client Client,
	container ContainerConfig,
	policy ContainerPolicy,
) (bool, error) {
	image, _, errInspect := client.client.ImageInspectWithRaw(ctx, policy.Image)
	if errInspect != nil {
		return false, fmt.Errorf("find image with label=%q: %w", policy.Image, errInspect)
	}

	imageID := image.ID

	return container.Name != policy.Name ||
		(policy.Hostname.Valid() && container.Hostname != policy.Hostname.Unwrap()) ||
		container.Image != imageID ||
		!elementsMatch(container.Networks, policy.Networks) ||
		!elementsMatch(container.Volumes, policy.Volumes) ||
		(policy.RestartPolicy.Valid() && container.RestartPolicy != policy.RestartPolicy.Unwrap()) ||
		(policy.Cmd.Valid() && !compareLists(policy.Cmd.Unwrap(), container.Cmd)), nil
}

func (c Client) ContainerStart(ctx context.Context, containerID ContainerID) error {
	return c.client.ContainerStart( //nolint:wrapcheck // lazy
		ctx,
		string(containerID),
		types.ContainerStartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		})
}

func (c Client) ImagePull(ctx context.Context, label string) error {
	r, errPull := c.client.ImagePull(ctx, label, types.ImagePullOptions{}) //nolint:exhaustruct // no options
	if errPull != nil {
		return fmt.Errorf("pull image: %w", errPull)
	}
	defer r.Close()

	resp, errLoad := c.client.ImageLoad(ctx, r, false)
	if errLoad != nil {
		return fmt.Errorf("load image: %w", errLoad)
	}
	defer resp.Body.Close()

	return nil
}

func (c Client) ContainerCreate(ctx context.Context, policy ContainerPolicy) (ContainerID, error) {
	if errPullImage := c.ImagePull(ctx, policy.Image); errPullImage != nil {
		return "", fmt.Errorf("pull image: %w", errPullImage)
	}

	var restartPolicy container.RestartPolicy
	if policy.RestartPolicy.Valid() {
		switch policy.RestartPolicy.Unwrap() {
		case RestartPolicyAlways:
			restartPolicy.Name = "always"
		case RestartPolicyOnFailure:
			restartPolicy.Name = "on-failure"
		case RestartPolicyUnlessStopped:
			restartPolicy.Name = "unless-stopped"
		default:
			return "", fmt.Errorf("unknown restart policy in container policy: %v", policy.RestartPolicy.Unwrap())
		}
	}

	container, errCreate := c.client.ContainerCreate(
		ctx,
		&container.Config{ //nolint:exhaustruct // not all options are needed
			Hostname: policy.Hostname.OrDefault(""),
			Image:    policy.Image,
			Volumes: fun.ToMap(policy.Volumes, func(volume string) (string, struct{}) {
				return volume, struct{}{}
			}),
			Cmd:          policy.Cmd.OrDefault(nil),
			Env:          []string{},    // TODO: fill from arg
			ExposedPorts: nat.PortSet{}, // TODO: fill from arg
		},
		&container.HostConfig{ //nolint:exhaustruct // not all options are needed
			RestartPolicy: restartPolicy,
		},
		nil, // TODO: fill networks from cfg
		nil,
		policy.Name,
	)
	if errCreate != nil {
		return "", fmt.Errorf("create container: %w", errCreate)
	}

	return ContainerID(container.ID), nil
}

func (c Client) ContainerRun(ctx context.Context, policy ContainerPolicy) error {
	containerID, errCreate := c.ContainerCreate(ctx, policy)
	if errCreate != nil {
		return fmt.Errorf("run, create container: %w", errCreate)
	}

	if errStart := c.ContainerStart(ctx, containerID); errStart != nil {
		return fmt.Errorf("run, start container: %w", errStart)
	}

	return nil
}

func (c Client) ContainerStop(ctx context.Context, containerID ContainerID) error {
	if errStop := c.client.ContainerStop(ctx, string(containerID), nil); errStop != nil {
		return fmt.Errorf("stop container %q: %w", containerID, errStop)
	}
	return nil
}

func (c Client) ContainerRemove(ctx context.Context, containerID ContainerID) error {
	return c.client.ContainerRemove( //nolint:wrapcheck // lazy
		ctx,
		string(containerID),
		types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         false,
			RemoveLinks:   false,
		})
}

// TODO: use

func (c Client) ContainerRestart(ctx context.Context, containerID ContainerID) error {
	// TODO: do in one docker client call
	if errStop := c.ContainerStop(ctx, containerID); errStop != nil {
		return fmt.Errorf("restart container, stopping: %w", errStop)
	}

	if errStart := c.ContainerStart(ctx, containerID); errStart != nil {
		// TODO: add container info to errors
		return fmt.Errorf("restart container, starting: %w", errStart)
	}

	return nil
}

// TODO: return actions list instead
func Reconcile( //nolint:funlen,gocognit,cyclop // fuckyou
	ctx context.Context,
	client Client,
	policy ContainerPolicy,
	containerMaybe fun.Option[ContainerConfig],
) error {
	if !containerMaybe.Valid() {
		if errRun := client.ContainerRun(ctx, policy); errRun != nil {
			return fmt.Errorf("no container found, creating and running it: %w", errRun)
		}
	}

	container := containerMaybe.Unwrap()

	recreate, err := needsRecreate(ctx, client, container, policy)
	if err != nil {
		return fmt.Errorf("check needs recreate: %w", err)
	} else if recreate {
		// TODO: if remove stops container, it doesn't need to be stopped beforehand
		return nil // TODO: recreate = stop, remove, run
	}

	switch policy.State {
	case ContainerDesiredStateStarted:
		switch container.State {
		case ContainerStateRunning, ContainerStateRestarting:
			return nil
		case ContainerStateCreated, ContainerStatePaused, ContainerStateExited:
			if errRun := client.ContainerStart(ctx, container.ID); errRun != nil {
				return fmt.Errorf("container %s stopped, running it: %w", container.ID, errRun)
			}
			return nil
		case ContainerStateRemoving:
			if errStart := client.ContainerRun(ctx, policy); errStart != nil {
				return fmt.Errorf("container %s removed, starting it: %w", container.ID, errStart)
			}
			return nil
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
			if errRemove := client.ContainerRemove(ctx, container.ID); errRemove != nil {
				return fmt.Errorf("container %s present, removing it: %w", container.ID, errRemove)
			}
			return nil
		case ContainerStateRunning, ContainerStateRestarting:
			if errStop := client.ContainerStop(ctx, container.ID); errStop != nil {
				return fmt.Errorf("container %s running, stopping it: %w", container.ID, errStop)
			}
			if errRemove := client.ContainerRemove(ctx, container.ID); errRemove != nil {
				return fmt.Errorf("container %s running, removing it: %w", container.ID, errRemove)
			}
			return nil
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
			if containerID, errCreate := client.ContainerCreate(ctx, policy); errCreate != nil {
				return fmt.Errorf("container %s removed, creating it with id=%s: %w", container.ID, containerID, errCreate)
			}
			return nil
		default:
			return fmt.Errorf("don't know how to become present from state %v", container.State)
		}
	case ContainerDesiredStateStopped:
		switch container.State {
		case ContainerStateCreated, ContainerStatePaused, ContainerStateRemoving, ContainerStateExited:
			return nil
		case ContainerStateRunning, ContainerStateRestarting:
			if errStop := client.ContainerStop(ctx, container.ID); errStop != nil {
				return fmt.Errorf("container %s running, stopping it: %w", container.ID, errStop)
			}
			return nil
		case ContainerStateDead:
			return fmt.Errorf("don't know how to stop from dead state")
		default:
			return fmt.Errorf("don't know how to stop from state %v", container.State)
		}
	default:
		return fmt.Errorf("unknown container desired state: %v", policy.State)
	}
}

type PolicyMatch struct {
	Policy    ContainerPolicy
	Container fun.Option[ContainerConfig]
}

func MatchPolicies(containers map[ContainerID]ContainerConfig, policies []ContainerPolicy) []PolicyMatch {
	containersByName := make(map[string]ContainerConfig, len(containers))
	for _, container := range containers {
		containersByName[container.Name] = container
	}

	res := make([]PolicyMatch, len(policies))
	for i, policy := range policies {
		container, ok := containersByName[policy.Name]
		res[i] = PolicyMatch{
			Policy:    policy,
			Container: fun.Optional(container, ok),
		}
	}
	return res
}
