package worker

import (
	"errors"
	"fmt"
	"github.com/MarouaneBouaricha/cube/api/stats"
	store2 "github.com/MarouaneBouaricha/cube/internal/store"
	task2 "github.com/MarouaneBouaricha/cube/internal/task"
	"log"
	"time"

	"github.com/golang-collections/collections/queue"
)

type Status int

const (
	Ready Status = iota
	NotReady
)

type Worker struct {
	Name             string
	Queue            queue.Queue
	Db               store2.Store
	Stats            *stats.Stats
	TaskCount        int
	Status           Status
	ContainerRuntime task2.ContainerRuntime
}

func New(name string, taskDbType string, containerRuntime string) *Worker {
	w := Worker{
		Name:  name,
		Queue: *queue.New(),
	}

	var s store2.Store
	var err error
	switch taskDbType {
	case "memory":
		s = store2.NewInMemoryTaskStore()
	case "persistent":
		filename := fmt.Sprintf("%s_tasks.db", name)
		s, err = store2.NewTaskStore(filename, 0600, "tasks")
	}
	if err != nil {
		log.Printf("unable to create new task store: %v", err)
	}

	switch containerRuntime {
	case "docker":
		err = task2.DaemonHealthCheck()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Docker is Running!")
	}
	w.Db = s
	return &w
}

func (w *Worker) GetTasks() []*task2.Task {
	taskList, err := w.Db.List()
	if err != nil {
		log.Printf("error getting list of tasks: %v\n", err)
		return nil
	}

	return taskList.([]*task2.Task)
}

func (w *Worker) CollectStats() {
	for {
		log.Println("Collecting stats")
		w.Stats = stats.GetStats()
		w.TaskCount = w.Stats.TaskCount
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) AddTask(t task2.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) RunTasks() {
	for {
		if w.Queue.Len() != 0 {
			result := w.runTask()
			if result.Error != nil {
				log.Printf("Error running task: %v\n", result.Error)
			}
		} else {
			log.Printf("No tasks to process currently.\n")
		}
		log.Println("Sleeping for 10 seconds.")
		time.Sleep(10 * time.Second)
	}

}

func (w *Worker) runTask() task2.ContainerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("[worker] No tasks in the queue")
		return task2.ContainerResult{Error: nil}
	}

	taskQueued := t.(task2.Task)
	fmt.Printf("[worker] Found task in queue: %v:\n", taskQueued)

	err := w.Db.Put(taskQueued.ID.String(), &taskQueued)
	if err != nil {
		msg := fmt.Errorf("error storing task %s: %v", taskQueued.ID.String(), err)
		log.Println(msg)
		return task2.ContainerResult{Error: msg}
	}

	queuedTask, err := w.Db.Get(taskQueued.ID.String())
	if err != nil {
		msg := fmt.Errorf("error getting task %s from database: %v", taskQueued.ID.String(), err)
		log.Println(msg)
		return task2.ContainerResult{Error: msg}
	}

	taskPersisted := *queuedTask.(*task2.Task)

	if taskPersisted.State == task2.Completed {
		return w.StopTask(taskPersisted)
	}

	var containerResult task2.ContainerResult
	if task2.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task2.Scheduled:
			if taskQueued.ContainerID != "" {
				containerResult = w.StopTask(taskQueued)
				if containerResult.Error != nil {
					log.Printf("%v\n", containerResult.Error)
				}
			}
			containerResult = w.StartTask(taskQueued)
		case task2.Completed:
			containerResult = w.StopTask(taskQueued)
		default:
			fmt.Printf("This is a mistake. taskPersisted: %v, taskQueued: %v\n", taskPersisted, taskQueued)
			containerResult.Error = errors.New("we should not get here")
		}
	} else {
		err := fmt.Errorf("invalid transition from %v to %v", taskPersisted.State, taskQueued.State)
		containerResult.Error = err
		return containerResult
	}
	return containerResult
}

func (w *Worker) StartTask(t task2.Task) task2.ContainerResult {
	config := task2.NewConfig(&t)
	w.ContainerRuntime = task2.NewDocker(config)
	result := w.ContainerRuntime.Run()
	if result.Error != nil {
		log.Printf("Err running task %v: %v\n", t.ID, result.Error)
		t.State = task2.Failed
		w.Db.Put(t.ID.String(), &t)
		return result
	}

	t.ContainerID = result.ContainerId
	t.State = task2.Running
	w.Db.Put(t.ID.String(), &t)

	return result
}

func (w *Worker) StopTask(t task2.Task) task2.ContainerResult {
	config := task2.NewConfig(&t)
	w.ContainerRuntime = task2.NewDocker(config)

	stopResult := w.ContainerRuntime.Stop(t.ContainerID)
	if stopResult.Error != nil {
		log.Printf("%v\n", stopResult.Error)
	}
	removeResult := w.ContainerRuntime.Remove(t.ContainerID)
	if removeResult.Error != nil {
		log.Printf("%v\n", removeResult.Error)
	}

	t.FinishTime = time.Now().UTC()
	t.State = task2.Completed
	w.Db.Put(t.ID.String(), &t)
	log.Printf("Stopped and removed container %v for task %v\n", t.ContainerID, t.ID)

	return removeResult
}

func (w *Worker) InspectTask(t task2.Task) task2.ContainerInspectResponse {
	config := task2.NewConfig(&t)
	w.ContainerRuntime = task2.NewDocker(config)
	return w.ContainerRuntime.Inspect(t.ContainerID)
}

func (w *Worker) UpdateTasks() {
	for {
		log.Println("Checking status of tasks")
		w.updateTasks()
		log.Println("Task updates completed")
		log.Println("Sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) updateTasks() {
	tasks, err := w.Db.List()
	if err != nil {
		log.Printf("error getting list of tasks: %v\n", err)
		return
	}
	for _, t := range tasks.([]*task2.Task) {
		if t.State == task2.Running {
			resp := w.InspectTask(*t)
			if resp.Error != nil {
				fmt.Printf("ERROR: %v\n", resp.Error)
			}

			if resp.Container == nil {
				log.Printf("No container for running task %s\n", t.ID)
				t.State = task2.Failed
				w.Db.Put(t.ID.String(), t)
			}

			if resp.Container.State.Status == "exited" {
				log.Printf("Container for task %s in non-running state %s\n", t.ID, resp.Container.State.Status)
				t.State = task2.Failed
				w.Db.Put(t.ID.String(), t)
			}

			// task is running, update exposed ports
			t.HostPorts = resp.Container.NetworkSettings.NetworkSettingsBase.Ports
			w.Db.Put(t.ID.String(), t)
		}
	}
}
