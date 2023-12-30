package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/rprtr258/fun"
	"github.com/rs/zerolog/log"
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

func (state ContainerState) String() string {
	switch state {
	case ContainerStateCreated:
		return "created"
	case ContainerStateRunning:
		return "running"
	case ContainerStatePaused:
		return "paused"
	case ContainerStateRestarting:
		return "restarting"
	case ContainerStateRemoving:
		return "removing"
	case ContainerStateExited:
		return "exited"
	case ContainerStateDead:
		return "dead"
	default:
		return "unknown"
	}
}

type ContainerID string

type ContainerConfig struct {
	ID            ContainerID
	Name          string
	Hostname      string
	Image         string
	Networks      []string
	Volumes       []Mount
	RestartPolicy RestartPolicy
	State         ContainerState
	Cmd           []string
	Env           map[string]string
	PortBindings  map[nat.Port][]nat.PortBinding
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
		case "created":
			state = ContainerStateCreated
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
			Image:    details.Image,                              // TODO: image hash
			Networks: fun.Keys(details.NetworkSettings.Networks), // TODO: fill
			Volumes: fun.Map[Mount](func(m types.MountPoint) Mount {
				return Mount{
					Source:   m.Source,
					Target:   m.Destination,
					ReadOnly: !m.RW,
				}
			}, details.Mounts...),
			RestartPolicy: restartPolicy,
			State:         state,
			Cmd:           details.Config.Cmd,
			Env: fun.SliceToMap[string, string](
				func(env string) (string, string) {
					const _partsNumber = 2 // 2 parts: key=value

					parts := strings.SplitN(env, "=", _partsNumber)
					if len(parts) != _partsNumber {
						return parts[0], ""
					}

					return parts[0], parts[1]
				},
				details.Config.Env...,
			),
			PortBindings: details.HostConfig.PortBindings,
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

type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type ContainerPolicy struct {
	Name          string
	Hostname      fun.Option[string]
	Image         string
	Networks      []string
	Volumes       []Mount
	RestartPolicy fun.Option[RestartPolicy]
	State         ContainerDesiredState
	Cmd           fun.Option[[]string]
	Env           map[string]string
	PortBindings  map[nat.Port][]nat.PortBinding
}

func elementsMatch[T comparable](xs, ys []T) bool {
	if len(xs) != len(ys) {
		return false
	}

	aSet := fun.SliceToMap[T, struct{}](func(elem T) (T, struct{}) {
		return elem, struct{}{}
	}, xs...)

	return fun.All(func(s T) bool {
		_, ok := aSet[s]
		return ok
	}, ys...)
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

func compareMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if v != b[k] {
			return false
		}
	}

	return true
}

func compareMapsOfSlices(a, b map[nat.Port][]nat.PortBinding) bool {
	if len(a) != len(b) {
		return false
	}

	for k, av := range a {
		bv := b[k]
		if len(av) != len(bv) {
			return false
		}

		for i, avi := range av {
			if avi != bv[i] {
				return false
			}
		}
	}

	return true
}

func needsRecreate(
	ctx context.Context,
	client Client,
	container ContainerConfig,
	policy ContainerPolicy,
) (map[string]string, error) {
	image, _, errInspect := client.client.ImageInspectWithRaw(ctx, policy.Image)
	if errInspect != nil {
		if !errdefs.IsNotFound(errInspect) {
			return nil, fmt.Errorf("find image with label=%q: %w", policy.Image, errInspect)
		}

		if errPull := client.ImagePull(ctx, policy.Image); errPull != nil {
			return nil, fmt.Errorf("image not found, pulling %q: %w", policy.Image, errPull)
		}
	}

	difference := map[string]string{}
	if container.Name != policy.Name {
		difference["name"] = fmt.Sprintf("expected=%q actual=%q", policy.Name, container.Name)
	}
	if policy.Hostname.Valid && container.Hostname != policy.Hostname.Value {
		difference["hostname"] = fmt.Sprintf(
			"expected=%q actual=%q",
			policy.Hostname.Value, container.Hostname,
		)
	}
	if container.Image != image.ID {
		difference["image"] = fmt.Sprintf("expected=%q actual=%q", image.ID, container.Image)
	}
	if !elementsMatch(container.Networks, policy.Networks) {
		difference["networks"] = fmt.Sprintf("expected=%v actual=%v", policy.Networks, container.Networks)
	}
	if !elementsMatch(container.Volumes, policy.Volumes) {
		difference["volumes"] = fmt.Sprintf("expected=%v actual=%v", policy.Volumes, container.Volumes)
	}
	if policy.RestartPolicy.Valid && container.RestartPolicy != policy.RestartPolicy.Value {
		difference["restart policy"] = fmt.Sprintf(
			"expected=%q actual=%q",
			policy.RestartPolicy.Value, container.RestartPolicy,
		)
	}
	if policy.Cmd.Valid && !compareLists(policy.Cmd.Value, container.Cmd) {
		difference["cmd"] = fmt.Sprintf("expected=%v actual=%v", policy.Cmd.Value, container.Cmd)
	}
	if policy.Env != nil && !compareMaps(container.Env, policy.Env) { // TODO: ignore PATH env if it is not specified explicitly
		difference["env"] = fmt.Sprintf("expected=%v actual=%v", policy.Env, container.Env)
	}
	if !compareMapsOfSlices(container.PortBindings, policy.PortBindings) {
		difference["port bindings"] = fmt.Sprintf("expected=%v actual=%v", policy.PortBindings, container.PortBindings)
	}

	return difference, nil
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
	if policy.RestartPolicy.Valid {
		switch policy.RestartPolicy.Value {
		case RestartPolicyAlways:
			restartPolicy.Name = "always"
		case RestartPolicyOnFailure:
			restartPolicy.Name = "on-failure"
		case RestartPolicyUnlessStopped:
			restartPolicy.Name = "unless-stopped"
		default:
			return "", fmt.Errorf("unknown restart policy in container policy: %v", policy.RestartPolicy.Value)
		}
	}

	container, errCreate := c.client.ContainerCreate(ctx,
		&container.Config{ //nolint:exhaustruct // not all options are needed
			Hostname: policy.Hostname.OrDefault(""),
			Image:    policy.Image,
			Cmd:      policy.Cmd.OrDefault(nil),
			Env: fun.MapToSlice(policy.Env, func(name, value string) string {
				return fmt.Sprintf("%s=%s", name, value)
			}),
			ExposedPorts: nat.PortSet{}, // TODO: fill from arg
		},
		&container.HostConfig{ //nolint:exhaustruct // not all options are needed
			RestartPolicy: restartPolicy,
			PortBindings:  policy.PortBindings,
			Mounts: fun.Map[mount.Mount](func(volume Mount) mount.Mount {
				return mount.Mount{
					Type:          mount.TypeBind,
					Source:        volume.Source,
					Target:        volume.Target,
					ReadOnly:      true,
					Consistency:   mount.ConsistencyFull,
					BindOptions:   nil,
					VolumeOptions: nil,
					TmpfsOptions:  nil,
				}
			}, policy.Volumes...),
		},
		&network.NetworkingConfig{ //nolint:exhaustruct // not all options are needed
			EndpointsConfig: fun.SliceToMap[string, *network.EndpointSettings](
				func(networkName string) (string, *network.EndpointSettings) {
					return networkName, &network.EndpointSettings{
						NetworkID: networkName,
					}
				}, policy.Networks...),
			// TODO: fill networks from cfg
		},
		nil,
		policy.Name,
	)
	if errCreate != nil {
		return "", fmt.Errorf("create container: %w", errCreate)
	}

	return ContainerID(container.ID), nil
}

func (c Client) ContainerStart(ctx context.Context, containerID ContainerID) error {
	log.Debug().Str("container_id", string(containerID)).Msg("starting container")
	return c.client.ContainerStart( //nolint:wrapcheck // lazy
		ctx,
		string(containerID),
		types.ContainerStartOptions{
			CheckpointID:  "",
			CheckpointDir: "",
		})
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
	log.Debug().Str("container_id", string(containerID)).Msg("stopping container")
	if errStop := c.client.ContainerStop(ctx, string(containerID), nil); errStop != nil {
		return fmt.Errorf("stop container %q: %w", containerID, errStop)
	}
	return nil
}

func (c Client) ContainerRemove(ctx context.Context, containerID ContainerID) error {
	log.Debug().Str("container_id", string(containerID)).Msg("removing container")
	if errRemove := c.client.ContainerRemove(
		ctx,
		string(containerID),
		types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         false,
			RemoveLinks:   false,
		}); errRemove != nil {
		return fmt.Errorf("remove container %q: %w", containerID, errRemove)
	}
	return nil
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

// TODO: return actions list instead, dry run
func Reconcile( //nolint:funlen,gocognit,cyclop,gocyclo // fuckyou
	ctx context.Context,
	client Client,
	policy ContainerPolicy,
	containerMaybe fun.Option[ContainerConfig],
) error {
	if !containerMaybe.Valid {
		if policy.State == ContainerDesiredStateAbsent {
			log.Info().Msg("no container found, as expected")
			return nil
		}

		log.Info().Msg("no container found, creating and running it")
		if errRun := client.ContainerRun(ctx, policy); errRun != nil {
			return fmt.Errorf("no container found, creating and running it: %w", errRun)
		}
	}

	container := containerMaybe.Value

	log.Info().
		Str("container_id", string(container.ID)).
		Str("container_state", container.State.String()).
		Int("policy_state", int(policy.State)).
		Msg("reconciling")

	diff, errCheckRecreate := needsRecreate(ctx, client, container, policy)
	if errCheckRecreate != nil {
		return fmt.Errorf("check needs recreate: %w", errCheckRecreate)
	}

	if len(diff) > 0 {
		log.Info().
			Str("container_id", string(container.ID)).
			Any("diff", diff).
			Msg("container needs to be recreated")

		if errStop := client.ContainerStop(ctx, container.ID); errStop != nil {
			return fmt.Errorf("stop old container: %w", errStop)
		}

		if errRemove := client.ContainerRemove(ctx, container.ID); errRemove != nil {
			return fmt.Errorf("remove old container: %w", errRemove)
		}

		newContainerID, errCreate := client.ContainerCreate(ctx, policy)
		if errCreate != nil {
			return fmt.Errorf("create container: %w", errCreate)
		}

		if policy.State == ContainerDesiredStateStarted {
			if errStart := client.ContainerStart(ctx, newContainerID); errStart != nil {
				return fmt.Errorf("start container: %w", errStart)
			}
		}

		return nil
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
			log.Info().
				Str("container_id", string(container.ID)).
				Msg("container running, stopping it")

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
