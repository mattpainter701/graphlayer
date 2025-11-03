package services

import (
	"fmt"
	"sync"
	"time"
)

// TaskService handles task tracking and status
type TaskService struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

// Task represents an async task
type Task struct {
	GUID      string    `json:"guid"`
	Scheduled bool      `json:"scheduled"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewTaskService creates a new task service
func NewTaskService() *TaskService {
	return &TaskService{
		tasks: make(map[string]*Task),
	}
}

// CreateTask creates a new task
func (ts *TaskService) CreateTask(guid string, scheduled bool) *Task {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task := &Task{
		GUID:      guid,
		Scheduled: scheduled,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	ts.tasks[guid] = task
	return task
}

// GetTask retrieves a task by GUID
func (ts *TaskService) GetTask(guid string) (*Task, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	task, exists := ts.tasks[guid]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", guid)
	}

	return task, nil
}

// UpdateTaskStatus updates the status of a task
func (ts *TaskService) UpdateTaskStatus(guid, status string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task, exists := ts.tasks[guid]
	if !exists {
		return fmt.Errorf("task not found: %s", guid)
	}

	task.Status = status
	task.UpdatedAt = time.Now()

	return nil
}

// UpdateTaskMetadata updates the metadata of a task
func (ts *TaskService) UpdateTaskMetadata(guid string, key string, value interface{}) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	task, exists := ts.tasks[guid]
	if !exists {
		return fmt.Errorf("task not found: %s", guid)
	}

	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}

	task.Metadata[key] = value
	task.UpdatedAt = time.Now()

	return nil
}

// DeleteTask removes a task from tracking
func (ts *TaskService) DeleteTask(guid string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if _, exists := ts.tasks[guid]; !exists {
		return fmt.Errorf("task not found: %s", guid)
	}

	delete(ts.tasks, guid)
	return nil
}

// ListTasks returns all tasks
func (ts *TaskService) ListTasks() []*Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	tasks := make([]*Task, 0, len(ts.tasks))
	for _, task := range ts.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// GetTasksByStatus returns tasks with a specific status
func (ts *TaskService) GetTasksByStatus(status string) []*Task {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var tasks []*Task
	for _, task := range ts.tasks {
		if task.Status == status {
			tasks = append(tasks, task)
		}
	}

	return tasks
}

