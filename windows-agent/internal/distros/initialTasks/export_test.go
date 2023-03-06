package initialTasks

import (
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
)

const InitialTasksFileName = initialTasksFileName

func (it *InitialTasks) Peek() []task.Task {
	return it.tasks
}
