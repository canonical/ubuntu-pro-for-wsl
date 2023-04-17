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

type ManagedTask = managedTask

//nolint:revive //unexported-return: Known false-positive: It is exported with an alias
func (w *Worker) TaskQueue() []*ManagedTask {
	return w.manager.tasks
}
