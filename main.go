package main

import (
	"fmt"
	"log"
)

type ResourceKeyKind int

const (
	ResourceKeyKindFile ResourceKeyKind = iota
	ResourceKeyKindPackage
	ResourceKeyKindTask
	ResourceKeyKindString
	ResourceKeyKindInt
)

func (kind ResourceKeyKind) String() string {
	switch kind {
	case ResourceKeyKindFile:
		return "file"
	case ResourceKeyKindPackage:
		return "package"
	case ResourceKeyKindTask:
		return "task"
	case ResourceKeyKindString:
		return "string"
	case ResourceKeyKindInt:
		return "int"
	default:
		return "<unknown>"
	}
}

type ResourceKey struct {
	Key  string
	Kind ResourceKeyKind
}

func (key ResourceKey) String() string {
	return fmt.Sprintf("#%s: %v", key.Key, key.Kind)
}

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

type Task struct {
	Description  Option[string]
	Dependencies []ResourceKey
	Produces     []ResourceKey
	Action       func([]Resource) ([]Resource, error)
}

type System struct {
	Tasks map[string]Task
	// TODO: separate by resource type?
	Resources map[ResourceKey]Resource
}

func Build(taskKey string, system System) ([]Resource, error) {
	log.Println("building", taskKey, "...")

	task, ok := system.Tasks[taskKey]
	if !ok {
		return nil, fmt.Errorf("%q task was not found\n", taskKey)
	}

	for _, kk := range task.Dependencies {
		switch kk.Kind {
		case ResourceKeyKindTask:
			if _, err := Build(kk.Key, system); err != nil {
				return nil, fmt.Errorf("build %q for %q: %w", kk.Key, taskKey, err)
			}
		}
	}

	dependencies := make([]Resource, len(task.Dependencies))
	for i, resourceKey := range task.Dependencies {
		resource, ok := system.Resources[resourceKey]
		if !ok {
			return nil, fmt.Errorf("resource %v not yet built/not found/can't be built", resourceKey)
		}

		dependencies[i] = resource
	}
	resources, err := task.Action(dependencies)
	if err != nil {
		return nil, fmt.Errorf("build %q: %w", taskKey, err)
	}

	for i, resourceKey := range task.Produces {
		system.Resources[resourceKey] = resources[i]
	}

	return resources, nil
}

func ShellAction(cmd string) func([]Resource) ([]Resource, error) {
	return func([]Resource) ([]Resource, error) {
		fmt.Printf("executing %q in shell...\n", cmd)
		return nil, nil
	}
}

func main() {
	example := System{
		Tasks:     map[string]Task{},
		Resources: map[ResourceKey]Resource{},
	}
	a := "aaakek"
	b := "aaalel"
	for i, c := range a {
		example.Resources[ResourceKey{Key: fmt.Sprintf("a%d", i), Kind: ResourceKeyKindString}] = StringResource(string(c))
	}
	for i, c := range b {
		example.Resources[ResourceKey{Key: fmt.Sprintf("b%d", i), Kind: ResourceKeyKindString}] = StringResource(string(c))
	}
	for i := 0; i < len(a); i++ {
		for j := 0; j < len(b); j++ {
			var task Task
			switch {
			case j == 0:
				task = Task{
					Description:  Option[string]{},
					Dependencies: nil,
					Produces:     []ResourceKey{{Key: fmt.Sprintf("c%d %d", i, j), Kind: ResourceKeyKindInt}},
					Action: func([]Resource) ([]Resource, error) {
						return []Resource{IntResource(i)}, nil
					},
				}
			case i == 0:
				task = Task{
					Description:  Option[string]{},
					Dependencies: nil,
					Produces:     []ResourceKey{{Key: fmt.Sprintf("c%d %d", i, j), Kind: ResourceKeyKindInt}},
					Action: func([]Resource) ([]Resource, error) {
						return []Resource{IntResource(j)}, nil
					},
				}
			default:
				task = Task{
					Description: Option[string]{},
					Dependencies: []ResourceKey{
						{Key: fmt.Sprintf("a%d", i), Kind: ResourceKeyKindString},
						{Key: fmt.Sprintf("b%d", j), Kind: ResourceKeyKindString},
						// TODO: change to int resources
						{Key: fmt.Sprintf("c%d %d", i-1, j-1), Kind: ResourceKeyKindTask}, // replace
						{Key: fmt.Sprintf("c%d %d", i, j-1), Kind: ResourceKeyKindTask},   // insert
						{Key: fmt.Sprintf("c%d %d", i-1, j), Kind: ResourceKeyKindTask},   // delete
					},
					Produces: []ResourceKey{{Key: fmt.Sprintf("c%d %d", i, j), Kind: ResourceKeyKindInt}},
					Action: func(rs []Resource) ([]Resource, error) {
						ac := rs[0].(StringResource)
						bc := rs[1].(StringResource)
						replace := rs[2].(IntResource)
						if ac == bc {
							return []Resource{IntResource(1 + int(replace))}, nil
						}

						insert := rs[3].(IntResource)
						delete := rs[4].(IntResource)

						/// min ///
						x := replace
						if insert < x {
							x = insert
						}
						if delete < x {
							x = delete
						}

						return []Resource{IntResource(1 + x)}, nil
					},
				}
			}
			example.Tasks[fmt.Sprintf("c%d %d", i, j)] = task
		}
	}
	for k, v := range example.Tasks {
		fmt.Println(k, v)
	}
	if _, err := Build("c5 5", example); err != nil {
		log.Fatal(err.Error())
	}

	// "compile": {
	// 	Docstring:    Option[string]{"build executable", true},
	// 	Dependencies: []Resource{FileResource("main.go")},
	// 	Produces:     []Resource{FileResource("mk")},
	// 	Action:       ShellAction("go build -o mk main.go"),
	// },
	// "run": {
	// 	Docstring:    Option[string]{"run main", true},
	// 	Dependencies: []Resource{TaskResource("compile")},
	// 	Produces:     nil,
	// 	Action:       ShellAction("./mk"),
	// },

	// fmt.Println("a::")
	// Build("a", example)

	// fmt.Println("\ncompile::")
	// Build("compile", example)

	// fmt.Println("\nrun::")
	// Build("run", example)
}
