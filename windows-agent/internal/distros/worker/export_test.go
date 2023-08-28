package worker

import (
	"fmt"
)

// TaskQueueSize is the number of tasks that can be enqueued.
const TaskQueueSize = taskQueueSize

// CheckQueuedTasks checks that the number of tasks in the queue matches expectations.
func (w *Worker) CheckQueuedTasks(want int) error {
	if got := w.manager.QueueLen(); got != want {
		return fmt.Errorf("Mismatch in number of queued tasks. Want: %d. Got: %d", want, got)
	}
	return nil
}

// CheckStoredTasks checks that the number of tasks in storage matches expectations.
func (w *Worker) CheckStoredTasks(want int) error {
	if got := w.manager.TaskLen(); got != want {
		return fmt.Errorf("Mismatch in number of stored tasks. Want: %d. Got: %d", want, got)
	}
	return nil
}
