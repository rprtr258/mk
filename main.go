package main

import (
	"fmt"
	"log"
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

type Task struct {
	Description  Option[string]
	Dependencies []any
	Produces     []any
	Action       func([]Resource) ([]Resource, error)
}

type System struct {
	Tasks map[string]Task
	// TODO: separate by resource type?
	Resources map[any]Resource
}

func Build(taskKey string, system System) ([]Resource, error) {
	log.Println("building", taskKey, "...")

	task, ok := system.Tasks[taskKey]
	if !ok {
		return nil, fmt.Errorf("%q task was not found\n", taskKey)
	}

	for _, dependency := range task.Dependencies {
		// TODO: optimize
		for _, resource := range system.Resources {
			if resource == dependency {
				// already done, skip
				goto DONE
			}
		}
		for tk, t := range system.Tasks {
			for _, tdep := range t.Produces {
				if tdep == dependency {
					if _, err := Build(tk, system); err != nil {
						return nil, fmt.Errorf("build %q for %q to get %v: %w", tk, taskKey, dependency, err)
					}
				}
			}
		}
	DONE:
	}

	dependencies := make([]Resource, 0, len(task.Dependencies))
	for _, resourceKey := range task.Dependencies {
		resource, ok := system.Resources[resourceKey]
		if !ok {
			return nil, fmt.Errorf("resource %v not yet built/not found/can't be built", resourceKey)
		}

		dependencies = append(dependencies, resource)
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
		Resources: map[any]Resource{},
	}
	type AKey int
	type BKey int
	type CKey [2]int
	a := "aaakek"
	b := "aaalel"
	for i, c := range a {
		example.Resources[AKey(i)] = StringResource(string(c))
	}
	for i, c := range b {
		example.Resources[BKey(i)] = StringResource(string(c))
	}
	for i := 0; i < len(a); i++ {
		i := i

		for j := 0; j < len(b); j++ {
			j := j

			var task Task
			switch {
			case j == 0:
				task = Task{
					Description:  Option[string]{},
					Dependencies: nil,
					Produces:     []any{CKey{i, j}},
					Action: func([]Resource) ([]Resource, error) {
						return []Resource{IntResource(i)}, nil
					},
				}
			case i == 0:
				task = Task{
					Description:  Option[string]{},
					Dependencies: nil,
					Produces:     []any{CKey{i, j}},
					Action: func([]Resource) ([]Resource, error) {
						return []Resource{IntResource(j)}, nil
					},
				}
			default:
				task = Task{
					Description: Option[string]{},
					Dependencies: []any{
						AKey(i),
						BKey(j),

						CKey{i - 1, j - 1}, // replace
						CKey{i, j - 1},     // insert
						CKey{i - 1, j},     // delete
					},
					Produces: []any{CKey{i, j}},
					Action: func(rs []Resource) ([]Resource, error) {
						ac := rs[0].(StringResource)
						bc := rs[1].(StringResource)
						replace := rs[2].(IntResource)
						if ac == bc {
							return []Resource{replace}, nil
						}

						insert := rs[3].(IntResource)
						delete := rs[4].(IntResource)

						/// x = min(replace, insert, delete) ///
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

	// for k, v := range example.Tasks {
	// 	fmt.Println(k, v)
	// }
	// fmt.Println()

	if _, err := Build("c5 5", example); err != nil {
		log.Fatal(err.Error())
	}

	// for k, v := range example.Resources {
	// 	fmt.Println(k, v)
	// }

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
