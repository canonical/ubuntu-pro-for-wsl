package task_test

import (
	"context"
	"os"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// We use the following number because is not representable as a float64:
//
//		0x20000000000001 -> float64 -> uint64 -> 0x20000000000000
//	                                          Loss of precision ^
//
// so if the yaml package does some intermediate conversion these tests will catch it.
// We've seen this happen when using an intermediate map[string]interface{}.
const bigInt uint64 = 0x20000000000001

func TestString(t *testing.T) {
	task := task.Managed{
		ID: 42,
		Task: testTask{
			Message: "Greetings",
			Number:  13,
		},
	}

	want := "Task #42 (task_test.testTask)"
	got := task.String()

	require.Equal(t, want, got, "String representation of Managed task does not match expectations")
}

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

//nolint: tparallel
// Cannot make test parallel because of BackupRegistry.
func TestMarshal(t *testing.T) {
	task.BackupRegistry(t)
	task.Register[testTask]()
	task.Register[emptyTask]()

	require.Implements(t, (*yaml.Marshaler)(nil), new(task.Managed), "task.Managed must implement yaml.Marshaler for the registry to work.")

	testCases := map[string]struct {
		input task.Managed
	}{
		"Simple task":                    {input: task.Managed{ID: 1, Task: testTask{Message: "Hello, world!", Number: 42}}},
		"Task with a line break":         {input: task.Managed{ID: 1234, Task: testTask{Message: "Hello, world!\nHow are you?", Number: 846531}}},
		"Task with a very large integer": {input: task.Managed{ID: 123456, Task: testTask{Message: "Not representable as a float64", Number: bigInt}}},
		"Task with no contents":          {input: task.Managed{ID: 9635, Task: emptyTask{}}},

		"Unregistered task should still marshal successfully": {input: task.Managed{ID: 9635, Task: unregisteredTask{Score: 9001}}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := yaml.Marshal(tc.input)
			require.NoError(t, err, "input task should marshal with no errors")

			// Avoiding LoadWithUpdateFromGoldenYAML to decouple marshalling and unmarshalling
			want := testutils.LoadWithUpdateFromGolden(t, string(got))

			require.Equal(t, want, string(got), "Task was not properly marshaled")
		})
	}
}

//nolint: tparallel
// Cannot make test parallel because of BackupRegistry.
func TestUnmarshal(t *testing.T) {
	task.BackupRegistry(t)
	task.Register[testTask]()
	task.Register[emptyTask]()

	require.Implements(t, (*yaml.Unmarshaler)(nil), new(task.Managed), "task.Managed must implement yaml.Unmarshaler for the registry to work.")

	testCases := map[string]struct {
		want    task.Managed
		wantErr bool
	}{
		"Simple task":                    {want: task.Managed{ID: 1, Task: testTask{Message: "Hello, world!", Number: 42}}},
		"Task with a line break":         {want: task.Managed{ID: 15, Task: testTask{Number: 64321, Message: "Hello, world!\nHow are you?"}}},
		"Task with a very large integer": {want: task.Managed{ID: 11, Task: testTask{Number: bigInt, Message: "Not representable as a float64"}}},
		"Empty task":                     {want: task.Managed{ID: 1997, Task: emptyTask{}}},

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

			data, err := os.ReadFile(testutils.TestFixturePath(t))
			t.Log(testutils.TestFixturePath(t))

			require.NoError(t, err, "Setup: could not find fixture")

			var got task.Managed
			err = yaml.Unmarshal(data, &got)
			if tc.wantErr {
				require.Error(t, err, "Task should return an error upon unmarshalling")
				return
			}

			require.NoError(t, err, "Registered task should not fail to unmarshal")
			require.Equal(t, tc.want, got, "Task was not properly unmarshaled")
		})
	}
}

//nolint: tparallel
// Cannot make test parallel because of BackupRegistry.
func TestMarsallUnmarshall(t *testing.T) {
	task.BackupRegistry(t)
	task.Register[testTask]()
	task.Register[emptyTask]()

	testCases := map[string]struct {
		input task.Managed
	}{
		"Simple task":                    {input: task.Managed{ID: 1, Task: testTask{Message: "Hello, world!", Number: 42}}},
		"Task with a line break":         {input: task.Managed{ID: 15, Task: testTask{Number: 64321, Message: "Hello, world!\nHow are you?"}}},
		"Task with a very large integer": {input: task.Managed{ID: 11, Task: testTask{Number: bigInt, Message: "Not representable as a float64"}}},
		"Empty task":                     {input: task.Managed{ID: 1997, Task: emptyTask{}}},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			serial, err := yaml.Marshal(tc.input)
			require.NoError(t, err, "input task should marshal with no errors")

			var got task.Managed
			err = yaml.Unmarshal(serial, &got)
			require.NoError(t, err, "Registered task should not fail to unmarshal")

			require.Equal(t, tc.input, got, "Marshaling, then unmarshaling a managed task should return the same object")
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
	DummyImplementer
}

type unregisteredTask struct {
	Score int

	DummyImplementer
}

// Boilerplate to implement the interface.
type DummyImplementer struct{}

func (DummyImplementer) Execute(context.Context, wslserviceapi.WSLClient) error {
	return nil
}

func (DummyImplementer) ShouldRetry() bool {
	return false
}
