package worker

// TaskQueueSize is the number of tasks that can be enqueued.
const TaskQueueSize = taskQueueSize

func (w *Worker) QueueLen() int {
	w.manager.mu.Lock()
	defer w.manager.mu.Unlock()

	return len(w.manager.queue)
}
