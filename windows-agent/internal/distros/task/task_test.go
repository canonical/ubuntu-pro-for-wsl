package task_test

import (
	"context"
	"os"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/common/golden"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/stretchr/testify/require"
)

// We use the following number because is not representable as a float64:
//
//		0x20000000000001 -> float64 -> uint64 -> 0x20000000000000
//	                                          Loss of precision ^
//
// so if the yaml package does some intermediate conversion these tests will catch it.
// We've seen this happen when using an intermediate map[string]interface{}.
const bigInt uint64 = 0x20000000000001

func TestRegistry(t *testing.T) {
	task.BackupRegistry(t)

	got := task.RegisteredTasks()
	require.Empty(t, got, "Setup: registry should be empty")

	task.Register[testTask]()
	task.Register[emptyTask]()

	want := []string{"task_test.testTask", "task_test.emptyTask"}
	got = task.RegisteredTasks()

	require.ElementsMatch(t, want, got, "registry should contain only the registered tasks")
}

//nolint:tparallel // Cannot make test parallel because of BackupRegistry.
func TestMarshal(t *testing.T) {
	task.BackupRegistry(t)
	task.Register[testTask]()
	task.Register[emptyTask]()

	testCases := map[string]struct {
		input task.Task
	}{
		"Simple task":                    {input: testTask{Message: "Hello, world!", Number: 42}},
		"Task with a line break":         {input: testTask{Message: "Hello, world!\nHow are you?", Number: 846531}},
		"Task with a very large integer": {input: testTask{Message: "Not representable as a float64", Number: bigInt}},
		"Task with no contents":          {input: emptyTask{}},

		"Unregistered task should still marshal successfully": {input: unregisteredTask{Score: 9001}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := task.MarshalYAML([]task.Task{tc.input})
			require.NoError(t, err, "input task should marshal with no errors")

			// Avoiding LoadWithUpdateFromGoldenYAML to decouple marshalling and unmarshalling
			want := golden.LoadWithUpdateFromGolden(t, string(got))

			require.Equal(t, want, string(got), "Task was not properly marshaled")
		})
	}
}

//nolint:tparallel // Cannot make test parallel because of BackupRegistry.
func TestUnmarshal(t *testing.T) {
	task.BackupRegistry(t)
	task.Register[testTask]()
	task.Register[emptyTask]()

	testCases := map[string]struct {
		want    task.Task
		wantErr bool
	}{
		"Simple task":                    {want: testTask{Message: "Hello, world!", Number: 42}},
		"Task with a line break":         {want: testTask{Number: 64321, Message: "Hello, world!\nHow are you?"}},
		"Task with a very large integer": {want: testTask{Number: bigInt, Message: "Not representable as a float64"}},
		"Empty task":                     {want: emptyTask{}},

		// Error cases
		"Error on unregistered task":    {wantErr: true},
		"Error on bad YAML syntax":      {wantErr: true},
		"Error on missing task label":   {wantErr: true},
		"Error on bad datatype in task": {wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(golden.TestFixturePath(t))
			t.Log(golden.TestFixturePath(t))

			require.NoError(t, err, "Setup: could not find fixture")

			got, err := task.UnmarshalYAML(data)
			if tc.wantErr {
				require.Error(t, err, "Task should return an error upon unmarshalling")
				return
			}
			require.NoError(t, err, "Registered task should not fail to unmarshal")

			require.Len(t, got, 1, "One and only one task was expected")
			require.Equal(t, tc.want, got[0], "Task was not properly unmarshaled")
		})
	}
}

//nolint:tparallel // Cannot make test parallel because of BackupRegistry.
func TestMarsallUnmarshall(t *testing.T) {
	task.BackupRegistry(t)
	task.Register[testTask]()
	task.Register[emptyTask]()

	testCases := map[string]struct {
		input task.Task
	}{
		"Simple task":                    {input: testTask{Message: "Hello, world!", Number: 42}},
		"Task with a line break":         {input: testTask{Number: 64321, Message: "Hello, world!\nHow are you?"}},
		"Task with a very large integer": {input: testTask{Number: bigInt, Message: "Not representable as a float64"}},
		"Empty task":                     {input: emptyTask{}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			serial, err := task.MarshalYAML([]task.Task{tc.input})
			require.NoError(t, err, "input task should marshal with no errors")

			got, err := task.UnmarshalYAML(serial)
			require.NoError(t, err, "Registered task should not fail to unmarshal")

			require.Len(t, got, 1, "One and only one task was expected")
			require.Equal(t, tc.input, got[0], "Marshaling, then unmarshaling a managed task should return the same object")
		})
	}
}

type testTask struct {
	Message string
	Number  uint64

	// we need to make this embedded struct to be public + ignore it instead of just using a private
	// embedded field so that the yaml package does not panic.
	DummyImplementer `yaml:"-"`
}

type emptyTask struct {
	DummyImplementer `yaml:"-"`
}

type unregisteredTask struct {
	Score int

	DummyImplementer `yaml:"-"`
}

// Boilerplate to implement the interface.
type DummyImplementer struct{}

func (DummyImplementer) Execute(context.Context, wslserviceapi.WSLClient) error {
	return nil
}
