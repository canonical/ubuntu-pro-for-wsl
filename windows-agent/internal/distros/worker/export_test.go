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

// WithStopCallback sets a function to be called once the Worker has stopped and
// task processing has stopped.
func WithStopCallback(f func()) Option {
	return func(o *options) {
		o.stopCallback = f
	}
}
