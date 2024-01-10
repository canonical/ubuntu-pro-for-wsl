package worker

import (
	"fmt"
)

// CheckQueuedTaskCount checks that the number of tasks in the queue matches expectations.
func (w *Worker) CheckQueuedTaskCount(want int) error {
	if got := w.manager.QueueLen(); got != want {
		return fmt.Errorf("Mismatch in number of queued tasks. Want: %d. Got: %d", want, got)
	}
	return nil
}

// CheckTotalTaskCount checks that the number of tasks in storage matches expectations.
func (w *Worker) CheckTotalTaskCount(want int) error {
	if got := w.manager.TaskLen(); got != want {
		return fmt.Errorf("Mismatch in number of stored tasks. Want: %d. Got: %d", want, got)
	}
	return nil
}
