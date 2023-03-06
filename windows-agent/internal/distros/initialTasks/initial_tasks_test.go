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
		"Success with no file":                 {},
		"Success with load from an empty file": {fileState: fileIsEmpty},
		"Success with load from file":          {fileState: fileHasValidTask, want: []task.Task{registeredTask{Data: "Hello!"}}},

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

			tasks := it.Peek()
			require.ElementsMatch(t, tc.want, tasks, "Mismatch between expected and obtained tasks after construction")
		})
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileStateBeforeNew saveFileState
		fileStateBeforeAdd saveFileState

		task task.Task

		wantErr bool
		want    []task.Task
	}{
		// Appending to empty tasklist
		"Success upon adding a first task when there is no save file":          {fileStateBeforeAdd: fileDoesNotExist, task: registeredTask{Data: "42"}, want: []task.Task{registeredTask{Data: "42"}}},
		"Success upon adding a first task when there is an empty save file":    {fileStateBeforeAdd: fileIsEmpty, task: registeredTask{Data: "42"}, want: []task.Task{registeredTask{Data: "42"}}},
		"Success upon adding a first task when there is a non-empty save file": {fileStateBeforeAdd: fileHasValidTask, task: registeredTask{Data: "42"}, want: []task.Task{registeredTask{Data: "42"}}},

		// Appending to non-empty tasklist
		"Success upon adding a second task when there is no save file":          {fileStateBeforeNew: fileHasValidTask, fileStateBeforeAdd: fileDoesNotExist, task: registeredTask{Data: "added task"}, want: []task.Task{registeredTask{Data: "loaded task"}, registeredTask{Data: "added task"}}},
		"Success upon adding a second task when there is an empty save file":    {fileStateBeforeNew: fileHasValidTask, fileStateBeforeAdd: fileIsEmpty, task: registeredTask{Data: "added task"}, want: []task.Task{registeredTask{Data: "loaded task"}, registeredTask{Data: "added task"}}},
		"Success upon adding a second task when there is a non-empty save file": {fileStateBeforeNew: fileHasValidTask, fileStateBeforeAdd: fileHasValidTask, task: registeredTask{Data: "added task"}, want: []task.Task{registeredTask{Data: "loaded task"}, registeredTask{Data: "added task"}}},

		// Error cases
		"Error upon adding a first task when the save file cannot be written":  {fileStateBeforeAdd: fileIsDirectory, task: registeredTask{Data: "42"}, wantErr: true},
		"Error upon adding a second task when the save file cannot be written": {fileStateBeforeNew: fileHasValidTask, fileStateBeforeAdd: fileIsDirectory, task: registeredTask{Data: "42"}, wantErr: true},
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

			err = it.Add(tc.task)
			if tc.wantErr {
				require.Error(t, err, "initalTasks.New should have returned an error")
				return
			}

			require.NoError(t, err, "initalTasks.Add should have returned no error")
			tasks := it.Peek()

			require.ElementsMatch(t, tc.want, tasks, "Mismatch between expected and obtained tasks after call to Add")
		})
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileStateBeforeNew    saveFileState
		fileStateBeforeRemove saveFileState

		task task.Task

		wantErr bool
		want    []task.Task
	}{
		// Appending to non-empty tasklist
		"Success upon removing a task when there is no save file":          {fileStateBeforeNew: fileHasValidTask, fileStateBeforeRemove: fileDoesNotExist, task: registeredTask{Data: "loaded task"}, want: []task.Task{}},
		"Success upon removing a task when there is an empty save file":    {fileStateBeforeNew: fileHasValidTask, fileStateBeforeRemove: fileIsEmpty, task: registeredTask{Data: "loaded task"}, want: []task.Task{}},
		"Success upon removing a task when there is a non-empty save file": {fileStateBeforeNew: fileHasValidTask, fileStateBeforeRemove: fileHasValidTask, task: registeredTask{Data: "loaded task"}, want: []task.Task{}},

		// Removeing a task that does not exist
		"Success upon removing a non-existent task when there is no save file":          {fileStateBeforeNew: fileHasValidTask, fileStateBeforeRemove: fileDoesNotExist, task: registeredTask{Data: "This task does not exist"}, want: []task.Task{registeredTask{Data: "loaded task"}}},
		"Success upon removing a non-existent task when there is an empty save file":    {fileStateBeforeNew: fileHasValidTask, fileStateBeforeRemove: fileIsEmpty, task: registeredTask{Data: "This task does not exist"}, want: []task.Task{registeredTask{Data: "loaded task"}}},
		"Success upon removing a non-existent task when there is a non-empty save file": {fileStateBeforeNew: fileHasValidTask, fileStateBeforeRemove: fileHasValidTask, task: registeredTask{Data: "This task does not exist"}, want: []task.Task{registeredTask{Data: "loaded task"}}},

		// Error cases
		"Error upon removing a task when the save file cannot be written": {fileStateBeforeNew: fileHasValidTask, fileStateBeforeRemove: fileIsDirectory, task: registeredTask{Data: "loaded task"}, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dir := t.TempDir()
			setUpSaveFile(t, tc.fileStateBeforeNew, dir)

			it, err := initialTasks.New(dir)
			require.NoError(t, err, "Setup: could not create initial tasks")

			setUpSaveFile(t, tc.fileStateBeforeRemove, dir)

			err = it.Remove(ctx, tc.task)
			if tc.wantErr {
				require.Error(t, err, "initalTasks.New should have returned an error")
				return
			}
			require.NoError(t, err, "initalTasks.Add should have returned no error")

			tasks := it.Peek()

			require.ElementsMatch(t, tc.want, tasks, "Mismatch between expected and obtained tasks after call to Remove")
		})
	}
}

func TestGetAll(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		fileState   saveFileState
		nilTaskList bool

		want []task.Task
	}{
		"Success upon getting an empty tasklist": {},
		"Success upon getting a single task":     {fileState: fileHasValidTask, want: []task.Task{registeredTask{Data: "loaded task"}}},
		"Success upon getting many tasks":        {fileState: fileHasThreeValidTasks, want: []task.Task{registeredTask{Data: "loaded task"}, registeredTask{Data: "loaded task"}, registeredTask{Data: "loaded task"}}},

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

			got := it.GetAll()
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
		out := taskfileFromTemplate[registeredTask](t)
		err := os.WriteFile(file, out, 0600)
		require.NoError(t, err, "Setup: could not create task file with a single task")
	case fileHasThreeValidTasks:
		removeFile()
		out := taskfileFromTemplate[registeredTask](t)
		err := os.WriteFile(file, bytes.Repeat(out, 3), 0600)
		require.NoError(t, err, "Setup: could not create task file with a single task")
	case fileHasNonRegisteredTask:
		removeFile()
		out := taskfileFromTemplate[unregisteredTask](t)
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

func taskfileFromTemplate[T task.Task](t *testing.T) []byte {
	t.Helper()

	in, err := os.ReadFile(filepath.Join(testutils.TestFamilyPath(t), "template.tasks"))
	require.NoError(t, err, "Setup: could not read tasks template")

	tmpl := template.Must(template.New(t.Name()).Parse(string(in)))

	w := &bytes.Buffer{}

	taskType := reflect.TypeOf((*T)(nil)).Elem().String()
	err = tmpl.Execute(w, taskType)
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
