package worker

// TaskQueueSize is the number of tasks that can be enqueued.
const TaskQueueSize = taskQueueSize

// QueueLen returns the number of tasks queued up. Any task currently being
// processed is not counted.
func (w *Worker) QueueLen() int {
	w.manager.mu.Lock()
	defer w.manager.mu.Unlock()

	return len(w.manager.queue)
}

// StorageLen returns the number of tasks in storage. Tasks currently being
// processed, and tasks that failed with NeedsRetryError will be counted.
func (w *Worker) StorageLen() int {
	w.manager.mu.Lock()
	defer w.manager.mu.Unlock()

	return len(w.manager.tasks)
}
