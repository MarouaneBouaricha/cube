package manager

import (
	nd "github.com/MarouaneBouaricha/cube/api/node"
	sc2 "github.com/MarouaneBouaricha/cube/internal/scheduler"
	st2 "github.com/MarouaneBouaricha/cube/internal/store"
	tk "github.com/MarouaneBouaricha/cube/internal/task"
	wkr "github.com/MarouaneBouaricha/cube/internal/worker"

	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	nt2 "github.com/docker/go-connections/nat"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)

type Manager struct {
	Pending       queue.Queue
	TaskDb        st2.Store
	EventDb       st2.Store
	Workers       []string
	WorkerTaskMap map[string][]uuid.UUID
	TaskWorkerMap map[uuid.UUID]string
	LastWorker    int
	WorkerNodes   []*nd.Node
	Scheduler     sc2.Scheduler
}

func New(workers []string, schedulerType string, dbType string) *Manager {
	workerTaskMap := make(map[string][]uuid.UUID)
	taskWorkerMap := make(map[uuid.UUID]string)

	var nodes []*nd.Node
	for worker := range workers {
		workerTaskMap[workers[worker]] = []uuid.UUID{}

		nAPI := fmt.Sprintf("http://%v", workers[worker])
		n := nd.NewNode(workers[worker], nAPI, "worker")
		nodes = append(nodes, n)
	}

	var s sc2.Scheduler
	switch schedulerType {
	case "epvm":
		s = &sc2.Epvm{Name: "epvm"}
	case "roundrobin":
		s = &sc2.RoundRobin{Name: "roundrobin"}
	default:
		s = &sc2.Epvm{Name: "epvm"}
	}

	m := Manager{
		Pending:       *queue.New(),
		Workers:       workers,
		WorkerTaskMap: workerTaskMap,
		TaskWorkerMap: taskWorkerMap,
		WorkerNodes:   nodes,
		Scheduler:     s,
	}

	var ts st2.Store
	var es st2.Store
	var esErr, tsErr error
	switch dbType {
	case "memory":
		ts = st2.NewInMemoryTaskStore()
		es = st2.NewInMemoryTaskEventStore()
	case "persistent":
		ts, tsErr = st2.NewTaskStore("tasks.db", 0600, "tasks")
		es, esErr = st2.NewEventStore("events.db", 0600, "events")
	}

	if tsErr != nil {
		log.Fatalf("unable to create task store: %v", tsErr)
	}

	if esErr != nil {
		log.Fatalf("unable to create task event store: %v", esErr)
	}

	m.TaskDb = ts
	m.EventDb = es
	return &m
}

func (m *Manager) SelectWorker(t tk.Task) (*nd.Node, error) {
	candidates := m.Scheduler.SelectCandidateNodes(t, m.WorkerNodes)
	if candidates == nil {
		msg := fmt.Sprintf("No available candidates match resource request for task %v", t.ID)
		err := errors.New(msg)
		return nil, err
	}
	scores := m.Scheduler.Score(t, candidates)
	if scores == nil {
		return nil, fmt.Errorf("no scores returned to task %v", t)
	}
	selectedNode := m.Scheduler.Pick(scores, candidates)

	return selectedNode, nil
}

func (m *Manager) UpdateTasks() {
	for {
		log.Println("Checking for task updates from workers")
		m.updateTasks()
		log.Println("Task updates completed")
		log.Println("Sleeping for 15 seconds")
		time.Sleep(15 * time.Second)
	}
}

func (m *Manager) updateTasks() {
	for _, worker := range m.Workers {
		log.Printf("Checking worker %v for task updates", worker)
		url := fmt.Sprintf("http://%s/tasks", worker)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("[manager] Error connecting to %v: %v", worker, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("[manager] Error sending request: %v", err)
			continue
		}

		d := json.NewDecoder(resp.Body)
		var tasks []*tk.Task
		err = d.Decode(&tasks)
		if err != nil {
			log.Printf("[manager] Error unmarshalling tasks: %s", err.Error())
		}

		for _, t := range tasks {
			log.Printf("[manager] Attempting to update task %v", t.ID)

			result, err := m.TaskDb.Get(t.ID.String())
			if err != nil {
				log.Printf("[manager] %s\n", err)
				continue
			}
			taskPersisted, ok := result.(*tk.Task)
			if !ok {
				log.Printf("cannot convert result %v to task.Task type\n", result)
				continue
			}

			if taskPersisted.State != t.State {
				taskPersisted.State = t.State
			}

			taskPersisted.StartTime = t.StartTime
			taskPersisted.FinishTime = t.FinishTime
			taskPersisted.ContainerID = t.ContainerID
			taskPersisted.HostPorts = t.HostPorts

			putTskErr := m.TaskDb.Put(taskPersisted.ID.String(), taskPersisted)
			if putTskErr != nil {
				log.Printf("[manager] error updating task %v: %v", taskPersisted.ID, putTskErr)
				return
			}
		}
	}
}

func (m *Manager) UpdateNodeStats() {
	for {
		for _, node := range m.WorkerNodes {
			log.Printf("Collecting stats for node %v", node.Name)
			_, err := node.GetStats()
			if err != nil {
				log.Printf("error updating node stats: %v", err)
			}
		}
		time.Sleep(15 * time.Second)
	}
}

func (m *Manager) DoHealthChecks() {
	for {
		log.Println("Performing task health check")
		m.doHealthChecks()
		log.Println("Task health checks completed")
		log.Println("Sleeping for 60 seconds")
		time.Sleep(60 * time.Second)
	}
}

func (m *Manager) doHealthChecks() {
	tasks := m.GetTasks()
	for _, t := range tasks {
		if t.State == tk.Running && t.RestartCount < 3 {
			err := m.checkTaskHealth(*t)
			if err != nil {
				if t.RestartCount < 3 {
					m.restartTask(t)
				}
			}
		} else if t.State == tk.Failed && t.RestartCount < 3 {
			m.restartTask(t)
		}
	}
}

func (m *Manager) restartTask(t *tk.Task) {
	w := m.TaskWorkerMap[t.ID]
	t.State = tk.Scheduled
	t.RestartCount++
	putTskErr := m.TaskDb.Put(t.ID.String(), t)
	if putTskErr != nil {
		log.Printf("error updating task %v: %v", t.ID, putTskErr)
		return
	}

	te := tk.TaskEvent{
		ID:        uuid.New(),
		State:     tk.Running,
		Timestamp: time.Now(),
		Task:      *t,
	}
	data, err := json.Marshal(te)
	if err != nil {
		log.Printf("Unable to marshal task object: %v.", t)
	}

	url := fmt.Sprintf("http://%s/tasks", w)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("[manager] Error connecting to %v: %v", w, err)
		m.Pending.Enqueue(t)
		return
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		e := wkr.ErrResponse{}
		err := d.Decode(&e)
		if err != nil {
			fmt.Printf("Error decoding response: %s\n", err.Error())
			return
		}
		log.Printf("Response error (%d): %s", e.HTTPStatusCode, e.Message)
		return
	}

	newTask := tk.Task{}
	err = d.Decode(&newTask)
	if err != nil {
		fmt.Printf("Error decoding response: %s\n", err.Error())
		return
	}
	log.Printf("[manager] response from worker: %#v\n", t)
}

func getHostPort(ports nt2.PortMap) *string {
	var k nt2.Port
	for k = range ports {
		return &ports[k][0].HostPort
	}
	return nil
}

func (m *Manager) checkTaskHealth(t tk.Task) error {
	log.Printf("Calling health check for task %s: %s\n", t.ID, t.HealthCheck)

	w := m.TaskWorkerMap[t.ID]
	hostPort := getHostPort(t.HostPorts)
	worker := strings.Split(w, ":")
	if hostPort == nil {
		log.Printf("Have not collected task %s host port yet. Skipping.\n", t.ID)
		return nil
	}
	url := fmt.Sprintf("http://%s:%s%s", worker[0], *hostPort, t.HealthCheck)
	log.Printf("Calling health check for task %s: %s\n", t.ID, url)
	resp, err := http.Get(url)
	if err != nil {
		msg := fmt.Sprintf("[manager] Error connecting to health check %s", url)
		log.Println(msg)
		return errors.New(msg)
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("Error health check for task %s did not return 200\n", t.ID)
		log.Println(msg)
		return errors.New(msg)
	}

	log.Printf("Task %s health check response: %v\n", t.ID, resp.StatusCode)

	return nil
}

func (m *Manager) ProcessTasks() {
	for {
		log.Println("Processing any tasks in the queue")
		m.SendWork()
		log.Println("Sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

func (m *Manager) stopTask(worker string, taskID string) {
	client := &http.Client{}
	url := fmt.Sprintf("http://%s/tasks/%s", worker, taskID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		log.Printf("error creating request to delete task %s: %v", taskID, err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("error connecting to worker at %s: %v", url, err)
		return
	}

	if resp.StatusCode != 204 {
		log.Printf("Error sending request: %v", err)
		return
	}

	log.Printf("task %s has been scheduled to be stopped", taskID)
}

func (m *Manager) SendWork() {
	if m.Pending.Len() > 0 {
		e := m.Pending.Dequeue()
		te := e.(tk.TaskEvent)
		err := m.EventDb.Put(te.ID.String(), &te)
		if err != nil {
			log.Printf("error attempting to store task event %s: %s\n", te.ID.String(), err)
		}
		log.Printf("Pulled %v off pending queue\n", te)

		taskWorker, ok := m.TaskWorkerMap[te.Task.ID]
		if ok {
			result, err := m.TaskDb.Get(te.Task.ID.String())
			if err != nil {
				log.Printf("unable to schedule task: %s\n", err)
				return
			}

			persistedTask, ok := result.(*tk.Task)
			if !ok {
				log.Printf("unable to convert task to task.Task type\n")
				return
			}

			if te.State == tk.Completed && tk.ValidStateTransition(persistedTask.State, te.State) {
				m.stopTask(taskWorker, te.Task.ID.String())
				return
			}

			log.Printf("invalid request: existing task %s is in state %v and cannot transition to the completed state\n", persistedTask.ID.String(), persistedTask.State)
			return
		}

		t := te.Task
		w, err := m.SelectWorker(t)
		if err != nil {
			log.Printf("error selecting worker for task %s: %v\n", t.ID, err)
			return
		}

		log.Printf("[manager] selected worker %s for task %s\n", w.Name, t.ID)

		m.WorkerTaskMap[w.Name] = append(m.WorkerTaskMap[w.Name], te.Task.ID)
		m.TaskWorkerMap[t.ID] = w.Name

		t.State = tk.Scheduled
		putTskErr := m.TaskDb.Put(t.ID.String(), &t)
		if putTskErr != nil {
			log.Printf("error storing task %s: %v\n", t.ID, putTskErr)
			return
		}

		data, err := json.Marshal(te)
		if err != nil {
			log.Printf("Unable to marshal task object: %v.\n", t)
		}

		url := fmt.Sprintf("http://%s/tasks", w.Name)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
		if err != nil {
			log.Printf("[manager] Error connecting to %v: %v\n", w, err)
			m.Pending.Enqueue(t)
			return
		}

		d := json.NewDecoder(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			e := wkr.ErrResponse{}
			err := d.Decode(&e)
			if err != nil {
				fmt.Printf("Error decoding response: %s\n\n", err.Error())
				return
			}
			log.Printf("Response error (%d): %s\n", e.HTTPStatusCode, e.Message)
			return
		}

		t = tk.Task{}
		err = d.Decode(&t)
		if err != nil {
			fmt.Printf("Error decoding response: %s\n", err.Error())
			return
		}
		w.TaskCount++
		log.Printf("[manager] received response from worker: %#v\n", t)
	} else {
		log.Println("No work in the queue")
	}
}

func (m *Manager) GetTasks() []*tk.Task {
	taskList, err := m.TaskDb.List()
	if err != nil {
		log.Printf("error getting list of tasks: %v\n", err)
		return nil
	}

	return taskList.([]*tk.Task)
}

func (m *Manager) AddTask(te tk.TaskEvent) {
	log.Printf("Add event %v to pending queue", te)
	m.Pending.Enqueue(te)
}
