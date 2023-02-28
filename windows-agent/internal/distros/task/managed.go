package task

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/ubuntu/decorate"
	"gopkg.in/yaml.v3"
)

type decodeFunc = func(*yaml.Node) (Task, error)

var registry = map[string]decodeFunc{}

// Register registers a task type to the gobal registry. This is needed to deserialize
// tasks. Call Register[YourTask] to the module's init in order to use it.
func Register[T Task]() {
	typename := reflect.TypeOf((*T)(nil)).Elem().String()
	registry[typename] = func(node *yaml.Node) (Task, error) {
		var t T
		err := node.Decode(&t)
		return t, err
	}
}

// Managed is a type that carries a task with it, with added metadata and functionality to
// serialize and deserialize.
type Managed struct {
	ID uint64
	Task
}

// unmarshalPayload is a helper struct used to delay unmarshalling a Managed
// task until we know the type of tasks. Its fields must be the same as Managed,
// except for the task which must be a rawTask.
type unmarshalPayload struct {
	ID   uint64
	Task rawTask
}

func (m Managed) String() string {
	return fmt.Sprintf("Task #%d (%T)", m.ID, m.Task)
}

// MarshalYAML overrides the marshalling behaviour of Managed so that
// the type of the underlying Task can be embedded.
func (m Managed) MarshalYAML() (interface{}, error) {
	// payload contains the same contents as Managed but without methods.
	// Without this, marshalling the anonymous would recurse indefinitely.
	type payload Managed

	return struct {
		Payload payload
		Type    string
	}{
		Payload: payload(m),
		Type:    fmt.Sprintf("%T", m.Task),
	}, nil
}

// UnmarshalYAML overrides the unmarshalling behaviour of Managed so that
// the type of the underlying Task can be read before parsing its contents.
func (m *Managed) UnmarshalYAML(node *yaml.Node) error {
	var temp struct {
		Type    string
		Payload unmarshalPayload
	}

	err := node.Decode(&temp)
	if err != nil {
		return fmt.Errorf("could not decode intermediate struct: %v", err)
	}

	m.ID = temp.Payload.ID

	m.Task, err = temp.Payload.Task.decode(temp.Type)
	if err != nil {
		return err
	}

	return nil
}

// rawTask is used to delay the unmarshalling of a Task. This is necessary because
// we don't know what type of task it is until we unmarshal part of the YAML document.
type rawTask struct {
	Node *yaml.Node
}

// UnmarshalYAML overrides the unmarshalling behaviour of rawTask so that it is not
// performed, and instead the node is stored. Once the type of task is known, use
// rawTask.decode(typename) to perform the unmarshalling.
func (rt *rawTask) UnmarshalYAML(node *yaml.Node) error {
	rt.Node = node
	return nil
}

// decode performs the actual unmarshalling. rawTask.UnmarshalYAML needs be called before.
func (rt rawTask) decode(taskTypeName string) (task Task, err error) {
	defer decorate.OnError(&err, "task type %q", taskTypeName)

	if rt.Node == nil {
		return nil, errors.New("decoding error: nil node")
	}

	decode, ok := registry[taskTypeName]
	if !ok {
		return nil, errors.New("not registered")
	}

	task, err = decode(rt.Node)
	if err != nil {
		return nil, fmt.Errorf("could not decode: %v", err)
	}
	return task, nil
}
