package mk

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Resource interface {
	isResource()
}

type FileResource string

func (FileResource) isResource() {}

type PackageResource struct {
	PackageManager string
	PackageName    string
	Version        string
}

func (PackageResource) isResource() {}

type TaskResource string

func (TaskResource) isResource() {}

type StringResource string

func (StringResource) isResource() {}

type IntResource int

func (IntResource) isResource() {}

type Option[T any] struct {
	Value T
	Valid bool
}

func (o Option[T]) String() string {
	if !o.Valid {
		return "None"
	}

	return fmt.Sprintf("Some(%v)", o.Value)
}

type ResourceKey fmt.Stringer
type Fetcher interface {
	Int(ResourceKey) (int, error)
	String(ResourceKey) (string, error)
}

type Task struct {
	Description Option[string]
	Produces    []ResourceKey
	Action      func(Fetcher) ([]Resource, error)
}

type System struct {
	// TODO: separate by resource type?
	Resources map[ResourceKey]Resource
	Tasks     map[string]Task
}

func (s System) bake(key ResourceKey) error {
	for taskKey2, task2 := range s.Tasks {
		for _, product := range task2.Produces {
			if product.String() == key.String() {
				// log.Printf("building %q to get %v for %q\n", taskKey2, key, taskKey)
				if _, err := s.Build(taskKey2); err != nil {
					// return 0, fmt.Errorf("build %q for %q to get %v: %w", taskKey2, taskKey, key, err)
					return fmt.Errorf("build %q to get %v: %w", taskKey2, key, err)
				}

				// TODO: check key is produced
				return nil
			}
		}
	}

	return nil
}

func (s System) Int(key ResourceKey) (int, error) {
	// TODO: optimize
	// already evaluated
	for resourceKey, resource := range s.Resources {
		if resourceKey.String() == key.String() {
			// already done, skip
			return int(resource.(IntResource)), nil
		}
	}

	if err := s.bake(key); err != nil {
		return 0, err
	}

	// TODO: remove duplication
	for resourceKey, resource := range s.Resources {
		if resourceKey.String() == key.String() {
			// already done, skip
			return int(resource.(IntResource)), nil
		}
	}

	return 0, fmt.Errorf("resource %v not yet built/not found/can't be built", key)
}

func (s System) String(key ResourceKey) (string, error) {
	// TODO: optimize
	// already evaluated
	for resourceKey, resource := range s.Resources {
		if resourceKey.String() == key.String() {
			// already done, skip
			return string(resource.(StringResource)), nil
		}
	}

	if err := s.bake(key); err != nil {
		return "", err
	}

	// TODO: remove duplication
	for resourceKey, resource := range s.Resources {
		if resourceKey.String() == key.String() {
			// already done, skip
			return string(resource.(StringResource)), nil
		}
	}

	return "", fmt.Errorf("resource %v not yet built/not found/can't be built", key)
}

func (s System) Build(taskKey string) ([]Resource, error) {
	task, ok := s.Tasks[taskKey]
	if !ok {
		return nil, fmt.Errorf("%q task was not found\n", taskKey)
	}

	resources, err := task.Action(s)
	if err != nil {
		return nil, fmt.Errorf("build %q: %w", taskKey, err)
	}

	for i, resourceKey := range task.Produces {
		s.Resources[resourceKey] = resources[i]
	}

	return resources, nil
}

func run(env map[string]string, stdout, stderr io.Writer, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Env = append(os.Environ(), MapToSlice(env, func(k, v string) string {
		return k + "=" + v
	})...)
	c.Stderr = stderr
	c.Stdout = stdout
	c.Stdin = os.Stdin

	if err := c.Run(); err != nil {
		return fmt.Errorf("cmd %q %v: %w", cmd, args, err)
	}

	return nil
}

func ShellCmd(cmd string, args ...string) (stdout string, stderr string, err error) {
	fmt.Printf("executing %q %v...\n", cmd, args)

	stdoutB := bytes.Buffer{}
	stderrB := bytes.Buffer{}
	if err := run(nil, &stdoutB, &stderrB, cmd, args...); err != nil {
		return "", "", err
	}

	return stdoutB.String(), stderrB.String(), nil
}

func ShellScript(script string) (stdout string, stderr string, err error) {
	return ShellCmd("/bin/sh", "-c", script)
}

func MapToSlice[K comparable, V, T any](dict map[K]V, f func(K, V) T) []T {
	res := make([]T, 0, len(dict))
	for k, v := range dict {
		res = append(res, f(k, v))
	}
	return res
}

func Map[T, R any](slice []T, f func(T) R) []R {
	res := make([]R, len(slice))
	for i, elem := range slice {
		res[i] = f(elem)
	}
	return res
}
