package task

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/ubuntu/decorate"
	"go.yaml.in/yaml/v3"
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

type yamlTaskHelper struct {
	Task Task
	Type string
}

// MarshalYAML marshals a slice of tasks in YAML format.
func MarshalYAML(tasks []Task) (out []byte, err error) {
	var tmp []yamlTaskHelper
	for i := range tasks {
		t := tasks[i]
		tmp = append(tmp, yamlTaskHelper{
			Type: reflect.TypeOf(t).String(),
			Task: t,
		})
	}

	return yaml.Marshal(tmp)
}

// UnmarshalYAML unmarshals a slice of tasks from a YAML document.
func UnmarshalYAML(in []byte) (tasks []Task, err error) {
	var tmp []yamlTaskHelper
	if err := yaml.Unmarshal(in, &tmp); err != nil {
		return nil, err
	}

	for i := range tmp {
		tasks = append(tasks, tmp[i].Task)
	}
	return tasks, nil
}

// UnmarshalYAML overrides the unmarshalling behaviour of yamlTaskHelper so that
// the type of the underlying Task can be read before parsing its contents.
func (t *yamlTaskHelper) UnmarshalYAML(node *yaml.Node) error {
	var tmp struct {
		Type string
		Task rawTask
	}

	err := node.Decode(&tmp)
	if err != nil {
		return fmt.Errorf("could not decode intermediate struct: %v", err)
	}

	t.Type = tmp.Type
	if t.Task, err = tmp.Task.decode(t.Type); err != nil {
		return err
	}

	return nil
}

// rawTask is used to delay the unmarshalling of a yamlTaskHelper. This is necessary because
// we don't know what type of task it contains is until we unmarshal part of the YAML document.
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
