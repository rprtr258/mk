package main

import "fmt"

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

type Task struct {
	Docstring    Option[string]
	Dependencies []Resource
	Produces     []Resource
	Action       func() error
}

type Tasks = map[string]Task

func Build(t string, tasks Tasks) error {
	task, ok := tasks[t]
	if !ok {
		return fmt.Errorf("%q task was not found\n", t)
	}

	for _, kk := range task.Dependencies {
		switch tt := kk.(type) {
		case TaskResource:
			if err := Build(string(tt), tasks); err != nil {
				return fmt.Errorf("build %q for %q: %w", tt, t, err)
			}
		}
	}

	if err := task.Action(); err != nil {
		return fmt.Errorf("build %q: %w", t, err)
	}

	return nil
}

func ShellAction(cmd string) func() error {
	return func() error {
		fmt.Printf("executing %q in shell...\n", cmd)
		return nil
	}
}

func main() {
	example := Tasks{
		"compile": {
			Docstring:    Option[string]{"build executable", true},
			Dependencies: []Resource{FileResource("main.go")},
			Produces:     []Resource{FileResource("mk")},
			Action:       ShellAction("go build -o mk main.go"),
		},
		"run": {
			Docstring:    Option[string]{"run main", true},
			Dependencies: []Resource{TaskResource("compile")},
			Produces:     nil,
			Action:       ShellAction("./mk"),
		},
	}

	fmt.Println("a::")
	Build("a", example)

	fmt.Println("\ncompile::")
	Build("compile", example)

	fmt.Println("\nrun::")
	Build("run", example)
}
