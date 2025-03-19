package task

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/moby/moby/pkg/stdcopy"
)

type Docker struct {
	Client *client.Client
	Config Config
}

func NewDocker(c *Config) *Docker {
	dc, initErr := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if initErr != nil {
		log.Fatalf("failed to initialize Docker client: %v", initErr)
	}

	_, err := dc.Ping(context.Background())
	if err != nil {
		log.Fatalf("Docker daemon is not running: %v", err)
	}
	return &Docker{
		Client: dc,
		Config: *c,
	}
}

func DaemonHealthCheck() error {
	dc, initErr := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if initErr != nil {
		return fmt.Errorf("failed to initialize Docker client: %v", initErr)
	}

	_, err := dc.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("Docker daemon is not running: %v", err)
	}
	return nil
}

type ContainerResult struct {
	Error       error
	Action      string
	ContainerId string
	Result      string
}

type ContainerInspectResponse struct {
	Error     error
	Container *types.ContainerJSON
}

func (d *Docker) Run() ContainerResult {
	ctx := context.Background()
	reader, err := d.Client.ImagePull(ctx, d.Config.Image, image.PullOptions{})
	if err != nil {
		log.Printf("Error pulling image %s: %v\n", d.Config.Image, err)
		return ContainerResult{Error: err}
	}
	io.Copy(os.Stdout, reader)

	rp := container.RestartPolicy{
		Name: d.Config.RestartPolicy,
	}

	r := container.Resources{
		Memory:   d.Config.Memory,
		NanoCPUs: int64(d.Config.Cpu * math.Pow(10, 9)),
	}

	hc := container.HostConfig{
		RestartPolicy:   rp,
		Resources:       r,
		PublishAllPorts: true,
	}

	resp, err := d.Client.ContainerCreate(ctx, &container.Config{
		Image:        d.Config.Image,
		Tty:          false,
		Env:          d.Config.Env,
		ExposedPorts: d.Config.ExposedPorts,
	}, &hc, nil, nil, d.Config.Name)
	if err != nil {
		log.Printf("Error creating container using image %s: %v\n", d.Config.Image, err)
		return ContainerResult{Error: err}
	}

	if err := d.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		log.Printf("Error starting container %s: %v\n", resp.ID, err)
		return ContainerResult{Error: err}
	}

	out, err := d.Client.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		log.Printf("Error getting logs for container %s: %v\n", resp.ID, err)
		return ContainerResult{Error: err}
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	return ContainerResult{ContainerId: resp.ID, Action: "start", Result: "success"}
}

func (d *Docker) Stop(id string) ContainerResult {
	log.Printf("Attempting to stop container %v", id)
	ctx := context.Background()
	err := d.Client.ContainerStop(ctx, id, container.StopOptions{})
	if err != nil {
		log.Printf("Error stopping container %s: %v\n", id, err)
		return ContainerResult{Error: err}
	}

	return ContainerResult{Action: "stop", Result: "success", Error: nil}
}

func (d *Docker) Remove(id string) ContainerResult {
	log.Printf("Attempting to delete container %v", id)
	ctx := context.Background()
	err := d.Client.ContainerRemove(ctx, id, container.RemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   false,
		Force:         false,
	})
	if err != nil {
		log.Printf("Error removing container %s: %v\n", id, err)
		return ContainerResult{Error: err}
	}
	return ContainerResult{Action: "delete", Result: "success", Error: nil}
}

func (d *Docker) Inspect(containerID string) ContainerInspectResponse {
	dc, _ := client.NewClientWithOpts(client.FromEnv)
	ctx := context.Background()
	resp, err := dc.ContainerInspect(ctx, containerID)
	if err != nil {
		log.Printf("Error inspecting container: %s\n", err)
		return ContainerInspectResponse{Error: err}
	}
	return ContainerInspectResponse{Container: &resp}

}
