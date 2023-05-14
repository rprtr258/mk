package docker

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

type ContainerPolicy struct {
	Name     string
	Hostname string
	Image    string
	Networks []struct {
		Name string
	}
	Volumes       []string
	RestartPolicy *container.RestartPolicy
}

func compareVolumes(containerVolumes map[string]struct{}, policyVolumes []string) bool {
	if len(containerVolumes) != len(policyVolumes) {
		return false
	}

	for _, volume := range policyVolumes {
		if _, ok := containerVolumes[volume]; !ok {
			return false
		}
	}

	return true
}

func NeedsRecreate(container types.ContainerJSON, policy ContainerPolicy) bool {
	return container.Name != policy.Name ||
		container.Config.Hostname != policy.Hostname ||
		container.Image != policy.Image ||
		// container.NetworkSettings.Networks != policy.Networks ||
		!compareVolumes(container.Config.Volumes, policy.Volumes) ||
		container.HostConfig.RestartPolicy.IsSame(policy.RestartPolicy)
}
