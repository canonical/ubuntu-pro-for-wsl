package initialTasks

import (
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
)

const InitialTasksFileName = initialTasksFileName

func (it *InitialTasks) Tasks() []task.Task {
	return it.tasks
}
