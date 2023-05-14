package main

import (
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
)

type Option[T any] struct {
	Value T
	Valid bool
}

type Port struct {
	// Host IP address that the container's port is mapped to
	IP Option[string]
	// Port on the container
	PrivatePort uint16
	// Port exposed on the host
	PublicPort Option[uint16]
	Type       string
}

// EndpointIPAMConfig represents IPAM configurations for the endpoint
type EndpointIPAMConfig struct {
	IPv4Address  string
	IPv6Address  string
	LinkLocalIPs []string
}

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	// Configurations
	IPAMConfig *EndpointIPAMConfig
	Links      []string
	Aliases    []string
	// Operational data
	NetworkID           string
	EndpointID          string
	Gateway             string
	IPAddress           string
	IPPrefixLen         int
	IPv6Gateway         string
	GlobalIPv6Address   string
	GlobalIPv6PrefixLen int
	MacAddress          string
	DriverOpts          map[string]string
}

type SummaryNetworkSettings struct {
	Networks map[string]*EndpointSettings
}

// MountType represents the type of a mount.
type MountType string

// Type constants
const (
	// TypeBind is the type for mounting host dir
	TypeBind MountType = "bind"
	// TypeVolume is the type for remote storage volumes
	TypeVolume MountType = "volume"
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs MountType = "tmpfs"
	// TypeNamedPipe is the type for mounting Windows named pipes
	TypeNamedPipe MountType = "npipe"
)

// MountPropagation represents the propagation of a mount.
type MountPropagation string

const (
	// PropagationRPrivate RPRIVATE
	PropagationRPrivate MountPropagation = "rprivate"
	// PropagationPrivate PRIVATE
	PropagationPrivate MountPropagation = "private"
	// PropagationRShared RSHARED
	PropagationRShared MountPropagation = "rshared"
	// PropagationShared SHARED
	PropagationShared MountPropagation = "shared"
	// PropagationRSlave RSLAVE
	PropagationRSlave MountPropagation = "rslave"
	// PropagationSlave SLAVE
	PropagationSlave MountPropagation = "slave"
)

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
type MountPoint struct {
	Type        MountType
	Name        string
	Source      string
	Destination string
	Driver      string
	Mode        string
	RW          bool
	Propagation MountPropagation
}

type HostConfig struct {
	NetworkMode string
}

type Container struct {
	ID              string
	Names           []string
	Image           string
	ImageID         string
	Command         string
	Created         int64
	Ports           []Port
	SizeRw          int64
	SizeRootFs      int64
	Labels          map[string]string
	State           string
	Status          string
	HostConfig      HostConfig
	NetworkSettings *SummaryNetworkSettings
	Mounts          []MountPoint
}

func MapPtr[T, R any](elem *T, f func(T) R) *R {
	if elem == nil {
		return nil
	}

	return fun.Ptr(f(*elem))
}

func main() {
	if err := (&cli.App{ //nolint:exhaustruct // daaaaa
		Name: "mk-agent",
		Commands: []*cli.Command{
			{
				Name:  "docker",
				Usage: "Get info about docker resources",
				Subcommands: []*cli.Command{
					{
						Name:  "containers",
						Usage: "Get info about docker containers",
						Action: func(ctx *cli.Context) error {
							client, err := dockerClient.NewClientWithOpts(dockerClient.FromEnv)
							if err != nil {
								return fmt.Errorf("docker client: %w", err)
							}

							containers, err := client.ContainerList(ctx.Context, types.ContainerListOptions{}) //nolint:exhaustruct,lll // no options
							if err != nil {
								return fmt.Errorf("list containers: %w", err)
							}

							for _, container := range containers {
								fmt.Printf("%#v\n", Container{
									ID:      container.ID,
									Names:   container.Names,
									Image:   container.Image,
									ImageID: container.ImageID,
									Command: container.Command,
									Created: container.Created,
									Ports: fun.Map(container.Ports, func(port types.Port) Port {
										return Port{
											IP: Option[string]{
												Value: port.IP,
												Valid: port.IP != "",
											},
											PrivatePort: port.PrivatePort,
											PublicPort: Option[uint16]{
												Value: port.PublicPort,
												Valid: port.PublicPort != 0,
											},
											Type: port.Type,
										}
									}),
									SizeRw:     container.SizeRw,
									SizeRootFs: container.SizeRootFs,
									Labels:     container.Labels,
									State:      container.State,
									HostConfig: HostConfig{
										NetworkMode: container.HostConfig.NetworkMode,
									},
									NetworkSettings: &SummaryNetworkSettings{
										Networks: lo.MapValues(
											container.NetworkSettings.Networks,
											func(settings *network.EndpointSettings, _ string) *EndpointSettings {
												return &EndpointSettings{
													IPAMConfig: MapPtr(settings.IPAMConfig, func(config network.EndpointIPAMConfig) EndpointIPAMConfig {
														return EndpointIPAMConfig{
															IPv4Address:  config.IPv4Address,
															IPv6Address:  config.IPv6Address,
															LinkLocalIPs: config.LinkLocalIPs,
														}
													}),
													Links:               settings.Links,
													Aliases:             settings.Aliases,
													NetworkID:           settings.NetworkID,
													EndpointID:          settings.EndpointID,
													Gateway:             settings.Gateway,
													IPAddress:           settings.IPAddress,
													IPPrefixLen:         settings.IPPrefixLen,
													IPv6Gateway:         settings.IPv6Gateway,
													GlobalIPv6Address:   settings.GlobalIPv6Address,
													GlobalIPv6PrefixLen: settings.GlobalIPv6PrefixLen,
													MacAddress:          settings.MacAddress,
													DriverOpts:          settings.DriverOpts,
												}
											},
										),
									},
									Mounts: fun.Map(container.Mounts, func(point types.MountPoint) MountPoint {
										return MountPoint{
											Type:        MountType(point.Type),
											Name:        point.Name,
											Source:      point.Source,
											Destination: point.Destination,
											Driver:      point.Driver,
											Mode:        point.Mode,
											RW:          point.RW,
											Propagation: MountPropagation(point.Propagation),
										}
									}),
								})
							}

							return nil
						},
					},
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
