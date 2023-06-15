package worker

import "fmt"

// TaskQueueSize is the number of tasks that can be enqueued.
const TaskQueueSize = taskQueueSize

// QueueLen returns the number of tasks queued up. Any task currently being
// processed is not counted.
func (w *Worker) CheckQueuedTasks(want int) error {
	w.manager.mu.Lock()
	defer w.manager.mu.Unlock()

	if got := len(w.manager.queue); got != want {
		return fmt.Errorf("Mismatch in number of queued tasks. Want: %d. Got: %d", want, got)
	}
	return nil
}

func (w *Worker) CheckStoredTasks(want int) error {
	w.manager.mu.Lock()
	defer w.manager.mu.Unlock()

	if got := len(w.manager.tasks); got != want {
		return fmt.Errorf("Mismatch in number of stored tasks. Want: %d. Got: %d", want, got)
	}
	return nil
}
