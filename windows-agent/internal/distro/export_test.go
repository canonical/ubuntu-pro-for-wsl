package distro

// FlushTaskQueue empties the task queue.
func (d *Distro) FlushTaskQueue() {
	tmp := make(chan Task, taskQueueBufferSize)
	d.tasks, tmp = tmp, d.tasks
	close(tmp)
}

const TaskQueueBufferSize = taskQueueBufferSize

func (d *Distro) QueueLen() int {
	return len(d.tasks)
}
