package worker

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/MarouaneBouaricha/cube/stats"
	"github.com/MarouaneBouaricha/cube/store"
	"github.com/MarouaneBouaricha/cube/task"
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
	Db               store.Store
	Stats            *stats.Stats
	TaskCount        int
	Status           Status
	ContainerRuntime task.ContainerRuntime
}

func New(name string, taskDbType string, containerRuntime string) *Worker {
	w := Worker{
		Name:  name,
		Queue: *queue.New(),
	}

	var s store.Store
	var err error
	switch taskDbType {
	case "memory":
		s = store.NewInMemoryTaskStore()
	case "persistent":
		filename := fmt.Sprintf("%s_tasks.db", name)
		s, err = store.NewTaskStore(filename, 0600, "tasks")
	}
	if err != nil {
		log.Printf("unable to create new task store: %v", err)
	}

	switch containerRuntime {
	case "docker":
		err = task.DaemonHealthCheck()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Docker is Running!")
	}
	w.Db = s
	return &w
}

func (w *Worker) GetTasks() []*task.Task {
	taskList, err := w.Db.List()
	if err != nil {
		log.Printf("error getting list of tasks: %v\n", err)
		return nil
	}

	return taskList.([]*task.Task)
}

func (w *Worker) CollectStats() {
	for {
		log.Println("Collecting stats")
		w.Stats = stats.GetStats()
		w.TaskCount = w.Stats.TaskCount
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) AddTask(t task.Task) {
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

func (w *Worker) runTask() task.ContainerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("[worker] No tasks in the queue")
		return task.ContainerResult{Error: nil}
	}

	taskQueued := t.(task.Task)
	fmt.Printf("[worker] Found task in queue: %v:\n", taskQueued)

	err := w.Db.Put(taskQueued.ID.String(), &taskQueued)
	if err != nil {
		msg := fmt.Errorf("error storing task %s: %v", taskQueued.ID.String(), err)
		log.Println(msg)
		return task.ContainerResult{Error: msg}
	}

	queuedTask, err := w.Db.Get(taskQueued.ID.String())
	if err != nil {
		msg := fmt.Errorf("error getting task %s from database: %v", taskQueued.ID.String(), err)
		log.Println(msg)
		return task.ContainerResult{Error: msg}
	}

	taskPersisted := *queuedTask.(*task.Task)

	if taskPersisted.State == task.Completed {
		return w.StopTask(taskPersisted)
	}

	var containerResult task.ContainerResult
	if task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			if taskQueued.ContainerID != "" {
				containerResult = w.StopTask(taskQueued)
				if containerResult.Error != nil {
					log.Printf("%v\n", containerResult.Error)
				}
			}
			containerResult = w.StartTask(taskQueued)
		case task.Completed:
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

func (w *Worker) StartTask(t task.Task) task.ContainerResult {
	config := task.NewConfig(&t)
	w.ContainerRuntime = task.NewDocker(config)
	result := w.ContainerRuntime.Run()
	if result.Error != nil {
		log.Printf("Err running task %v: %v\n", t.ID, result.Error)
		t.State = task.Failed
		w.Db.Put(t.ID.String(), &t)
		return result
	}

	t.ContainerID = result.ContainerId
	t.State = task.Running
	w.Db.Put(t.ID.String(), &t)

	return result
}

func (w *Worker) StopTask(t task.Task) task.ContainerResult {
	config := task.NewConfig(&t)
	w.ContainerRuntime = task.NewDocker(config)

	stopResult := w.ContainerRuntime.Stop(t.ContainerID)
	if stopResult.Error != nil {
		log.Printf("%v\n", stopResult.Error)
	}
	removeResult := w.ContainerRuntime.Remove(t.ContainerID)
	if removeResult.Error != nil {
		log.Printf("%v\n", removeResult.Error)
	}

	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.Db.Put(t.ID.String(), &t)
	log.Printf("Stopped and removed container %v for task %v\n", t.ContainerID, t.ID)

	return removeResult
}

func (w *Worker) InspectTask(t task.Task) task.ContainerInspectResponse {
	config := task.NewConfig(&t)
	w.ContainerRuntime = task.NewDocker(config)
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
	for _, t := range tasks.([]*task.Task) {
		if t.State == task.Running {
			resp := w.InspectTask(*t)
			if resp.Error != nil {
				fmt.Printf("ERROR: %v\n", resp.Error)
			}

			if resp.Container == nil {
				log.Printf("No container for running task %s\n", t.ID)
				t.State = task.Failed
				w.Db.Put(t.ID.String(), t)
			}

			if resp.Container.State.Status == "exited" {
				log.Printf("Container for task %s in non-running state %s\n", t.ID, resp.Container.State.Status)
				t.State = task.Failed
				w.Db.Put(t.ID.String(), t)
			}

			// task is running, update exposed ports
			t.HostPorts = resp.Container.NetworkSettings.NetworkSettingsBase.Ports
			w.Db.Put(t.ID.String(), t)
		}
	}
}
