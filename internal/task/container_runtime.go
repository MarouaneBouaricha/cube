package task

type ContainerRuntime interface {
	Run() ContainerResult
	Stop(id string) ContainerResult
	Remove(id string) ContainerResult
	Inspect(containerID string) ContainerInspectResponse
}
