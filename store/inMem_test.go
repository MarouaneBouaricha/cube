package store

import (
	"testing"
	"time"

	"github.com/MarouaneBouaricha/cube/task"
	"github.com/google/uuid"
)

func TestInMemoryTaskStore(t *testing.T) {
	store := NewInMemoryTaskStore()

	taskID := uuid.New()
	task1 := &task.Task{
		ID:          taskID,
		ContainerID: "container-1",
		Name:        "Task 1",
	}

	err := store.Put(taskID.String(), task1)
	if err != nil {
		t.Fatalf("Failed to put task: %v", err)
	}

	retrievedTask, err := store.Get(taskID.String())
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if retrievedTask.(*task.Task) != task1 {
		t.Errorf("Expected task %v, got %v", task1, retrievedTask)
	}

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	if len(tasks.([]*task.Task)) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks.([]*task.Task)))
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count tasks: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 task, got %d", count)
	}

	_, err = store.Get(uuid.New().String())
	if err == nil {
		t.Error("Expected error for non-existent key, got nil")
	}
}

func TestInMemoryTaskEventStore(t *testing.T) {
	store := NewInMemoryTaskEventStore()

	taskID := uuid.New()
	task1 := task.Task{
		ID:          taskID,
		ContainerID: "container-1",
		Name:        "Task 1",
	}
	eventID := uuid.New()
	event1 := &task.TaskEvent{
		ID:        eventID,
		Timestamp: time.Now(),
		Task:      task1,
	}

	err := store.Put(eventID.String(), event1)
	if err != nil {
		t.Fatalf("Failed to put task event: %v", err)
	}

	retrievedEvent, err := store.Get(eventID.String())
	if err != nil {
		t.Fatalf("Failed to get task event: %v", err)
	}
	if retrievedEvent.(*task.TaskEvent) != event1 {
		t.Errorf("Expected event %v, got %v", event1, retrievedEvent)
	}

	events, err := store.List()
	if err != nil {
		t.Fatalf("Failed to list task events: %v", err)
	}
	if len(events.([]*task.TaskEvent)) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events.([]*task.TaskEvent)))
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count task events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}

	_, err = store.Get(uuid.New().String())
	if err == nil {
		t.Error("Expected error for non-existent key, got nil")
	}
}
