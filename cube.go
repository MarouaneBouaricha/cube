package cube

import (
	aN "github.com/MarouaneBouaricha/cube/api/node"
	aS "github.com/MarouaneBouaricha/cube/api/stats"
	s "github.com/MarouaneBouaricha/cube/internal/store"
	t "github.com/MarouaneBouaricha/cube/internal/task"
	w "github.com/MarouaneBouaricha/cube/internal/worker"
)

// Here you put the logic that you want to expose to the world (types, methods, etc), as a library or package basic API's from your application.

type Node = aN.Node
type Stats = aS.Stats
type Worker = w.Worker
type Task = t.Task
type Store = s.Store

// NewWorker creates a new worker
func NewWorker(name string, taskDbType string, containerRuntime string) *Worker {
	return w.New(name, taskDbType, containerRuntime)
}

// NewNode creates a new node
func NewNode(name, api, role string) *Node {
	return aN.NewNode(name, api, role)
}

// NewConfig creates a new task
func NewConfig(task *t.Task) *t.Config {
	return t.NewConfig(task)
}

// NewDocker creates a new docker container runtime
func NewDocker(config *t.Config) *t.Docker {
	return t.NewDocker(config)
}

// And so on... (I just put some examples)
