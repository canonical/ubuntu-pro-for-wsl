package initialTasks_test

import (
	"bytes"
	"context"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/initialTasks"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/stretchr/testify/require"
)

func init() {
	task.Register[registeredTask]()
}

type saveFileState int

const (
	fileLeaveAsIs saveFileState = iota
	fileDoesNotExist
	fileIsEmpty
	fileHasValidTask
	fileHasThreeValidTasks
	fileHasNonRegisteredTask

	fileIsInvalid
	fileIsDirectory
)

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileState saveFileState

		wantErr bool
		want    []task.Task
	}{
		"Success with no file":                           {},
		"Success with load from registered task in file": {fileState: fileHasValidTask, want: []task.Task{registeredTask{Data: "loaded task 0"}}},
		"Success with load from an empty file":           {fileState: fileIsEmpty},

		"Error with load from file with non-registered task": {fileState: fileHasNonRegisteredTask, wantErr: true},
		"Error with load from file with invalid YAML":        {fileState: fileIsInvalid, wantErr: true},
		"Error with load from unreadable file":               {fileState: fileIsDirectory, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			setUpSaveFile(t, tc.fileState, dir)

			it, err := initialTasks.New(dir)
			if tc.wantErr {
				require.Error(t, err, "initialTasks.New should have returned an error")
				return
			}
			require.NoError(t, err, "initialTasks.New should have returned no error")

			got := it.Tasks()
			require.ElementsMatch(t, tc.want, got, "Mismatch between expected and obtained tasks after construction")
		})
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileStateBeforeNew saveFileState
		fileStateBeforeAdd saveFileState

		wantErr bool
		want    []task.Task
	}{
		// Appending to empty tasklist
		"Success upon adding a first task with no save file":          {fileStateBeforeAdd: fileDoesNotExist, want: []task.Task{registeredTask{Data: "added task"}}},
		"Success upon adding a first task with an empty save file":    {fileStateBeforeAdd: fileIsEmpty, want: []task.Task{registeredTask{Data: "added task"}}},
		"Success upon adding a first task with a non-empty save file": {fileStateBeforeAdd: fileHasValidTask, want: []task.Task{registeredTask{Data: "added task"}}},

		// Appending to non-empty tasklist
		"Success upon adding a second task with no save file":          {fileStateBeforeNew: fileHasValidTask, fileStateBeforeAdd: fileDoesNotExist, want: []task.Task{registeredTask{Data: "loaded task 0"}, registeredTask{Data: "added task"}}},
		"Success upon adding a second task with an empty save file":    {fileStateBeforeNew: fileHasValidTask, fileStateBeforeAdd: fileIsEmpty, want: []task.Task{registeredTask{Data: "loaded task 0"}, registeredTask{Data: "added task"}}},
		"Success upon adding a second task with a non-empty save file": {fileStateBeforeNew: fileHasValidTask, fileStateBeforeAdd: fileHasValidTask, want: []task.Task{registeredTask{Data: "loaded task 0"}, registeredTask{Data: "added task"}}},

		// Error cases
		"Error upon adding a first task with save file that cannot be written on":  {fileStateBeforeAdd: fileIsDirectory, wantErr: true},
		"Error upon adding a second task with save file that cannot be written on": {fileStateBeforeNew: fileHasValidTask, fileStateBeforeAdd: fileIsDirectory, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			setUpSaveFile(t, tc.fileStateBeforeNew, dir)

			it, err := initialTasks.New(dir)
			require.NoError(t, err, "Setup: could not create initial tasks")

			setUpSaveFile(t, tc.fileStateBeforeAdd, dir)

			err = it.Add(context.Background(), registeredTask{Data: "added task"})
			if tc.wantErr {
				require.Error(t, err, "initalTasks.New should have returned an error")
				return
			}

			require.NoError(t, err, "initalTasks.Add should have returned no error")
			tasks := it.Tasks()

			require.ElementsMatch(t, tc.want, tasks, "Add should have appended the expected new task to the list")
		})
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		task task.Task

		fileStateBeforeRemove saveFileState

		wantErr bool
		want    []task.Task
	}{
		// Appending to non-empty tasklist
		"Success upon removing a task with no save file":          {task: registeredTask{Data: "loaded task 0"}, fileStateBeforeRemove: fileDoesNotExist, want: []task.Task{}},
		"Success upon removing a task with an empty save file":    {task: registeredTask{Data: "loaded task 0"}, fileStateBeforeRemove: fileIsEmpty, want: []task.Task{}},
		"Success upon removing a task with a non-empty save file": {task: registeredTask{Data: "loaded task 0"}, fileStateBeforeRemove: fileHasValidTask, want: []task.Task{}},

		// Removing a task that does not exist
		"Success upon removing a non-existent task with no save file":          {task: registeredTask{Data: "This task does not exist"}, fileStateBeforeRemove: fileDoesNotExist, want: []task.Task{registeredTask{Data: "loaded task 0"}}},
		"Success upon removing a non-existent task with an empty save file":    {task: registeredTask{Data: "This task does not exist"}, fileStateBeforeRemove: fileIsEmpty, want: []task.Task{registeredTask{Data: "loaded task 0"}}},
		"Success upon removing a non-existent task with a non-empty save file": {task: registeredTask{Data: "This task does not exist"}, fileStateBeforeRemove: fileHasValidTask, want: []task.Task{registeredTask{Data: "loaded task 0"}}},

		// Error cases
		"Error upon removing a task when the save file cannot be written": {task: registeredTask{Data: "loaded task 0"}, fileStateBeforeRemove: fileIsDirectory, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dir := t.TempDir()
			setUpSaveFile(t, fileHasValidTask, dir)

			it, err := initialTasks.New(dir)
			require.NoError(t, err, "Setup: could not create initial tasks")

			setUpSaveFile(t, tc.fileStateBeforeRemove, dir)

			err = it.Remove(ctx, tc.task)
			if tc.wantErr {
				require.Error(t, err, "initalTasks.New should have returned an error")
				return
			}
			require.NoError(t, err, "initalTasks.Add should have returned no error")

			tasks := it.Tasks()

			require.ElementsMatch(t, tc.want, tasks, "Remove should have deleted the provided task from the list")
		})
	}
}

func TestAll(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileState   saveFileState
		nilTaskList bool

		want []task.Task
	}{
		"Success upon getting an empty tasklist": {},
		"Success upon getting a single task": {fileState: fileHasValidTask, want: []task.Task{
			registeredTask{Data: "loaded task 0"}}},
		"Success upon getting many tasks": {fileState: fileHasThreeValidTasks, want: []task.Task{
			registeredTask{Data: "loaded task 0"},
			registeredTask{Data: "loaded task 1"},
			registeredTask{Data: "loaded task 2"}}},

		"Success upon getting an empty tasklist from a nil initalTasks": {nilTaskList: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			setUpSaveFile(t, tc.fileState, dir)

			var it *initialTasks.InitialTasks
			if !tc.nilTaskList {
				i, err := initialTasks.New(dir)
				require.NoError(t, err, "Setup: could not create initial tasks")
				it = i
			}

			got := it.All()
			require.ElementsMatch(t, tc.want, got, "Mismatch between expected and obtained tasks after call to GetAll")
		})
	}
}

func setUpSaveFile(t *testing.T, fileState saveFileState, dir string) {
	t.Helper()

	file := filepath.Join(dir, initialTasks.InitialTasksFileName)

	removeFile := func() {
		t.Helper()

		err := os.Remove(file)
		if err != nil {
			require.ErrorIs(t, err, fs.ErrNotExist, "Setup: could not remove pre-existing task file")
		}
	}

	switch fileState {
	case fileLeaveAsIs:
	case fileDoesNotExist:
		removeFile()
	case fileIsEmpty:
		removeFile()
		f, err := os.Create(file)
		require.NoError(t, err, "Setup: could not create empty task file")
		f.Close()
	case fileHasValidTask:
		removeFile()
		out := taskfileFromTemplate[registeredTask](t, 1)
		err := os.WriteFile(file, out, 0600)
		require.NoError(t, err, "Setup: could not create task file with a single task")
	case fileHasThreeValidTasks:
		removeFile()
		out := taskfileFromTemplate[registeredTask](t, 3)
		err := os.WriteFile(file, out, 0600)
		require.NoError(t, err, "Setup: could not create task file with a single task")
	case fileHasNonRegisteredTask:
		removeFile()
		out := taskfileFromTemplate[unregisteredTask](t, 1)
		err := os.WriteFile(file, out, 0600)
		require.NoError(t, err, "Setup: could not create task file with a single task")
	case fileIsInvalid:
		removeFile()
		err := os.WriteFile(file, []byte("This is\n\tnot valid\n\t\tYAML"), 0600)
		require.NoError(t, err, "Setup: could not create invalid file with a single task")
	case fileIsDirectory:
		removeFile()
		err := os.MkdirAll(file, 0600)
		require.NoError(t, err, "Setup: could not create directory in place of the task file")
	default:
		require.Fail(t, "Setup: Unrecognized enum value for fileState", "Got: %d", fileState)
	}
}

func taskfileFromTemplate[T task.Task](t *testing.T, n int) []byte {
	t.Helper()

	in, err := os.ReadFile(filepath.Join(testutils.TestFamilyPath(t), "template.tasks"))
	require.NoError(t, err, "Setup: could not read tasks template")

	tmpl := template.Must(template.New(t.Name()).Parse(string(in)))

	w := &bytes.Buffer{}

	taskType := reflect.TypeOf((*T)(nil)).Elem().String()

	tasks := make([]string, n)
	for i := range tasks {
		tasks[i] = taskType
	}

	err = tmpl.Execute(w, tasks)
	require.NoError(t, err, "Setup: could not execute template task file")

	return w.Bytes()
}

type registeredTask struct {
	TaskBoilerplate `yaml:"-"`

	Data string
}

type unregisteredTask struct {
	TaskBoilerplate `yaml:"-"`

	Data int
}

// Boilerplate to implement the interface.
type TaskBoilerplate struct {
}

func (TaskBoilerplate) Execute(context.Context, wslserviceapi.WSLClient) error {
	return nil
}

func (TaskBoilerplate) ShouldRetry() bool {
	return false
}
