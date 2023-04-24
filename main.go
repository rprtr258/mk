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

type ResourceKey fmt.Stringer

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
	task, ok := system.Tasks[taskKey]
	if !ok {
		return nil, fmt.Errorf("%q task was not found\n", taskKey)
	}

	for _, dependency := range task.Dependencies {
		// TODO: optimize
		for resourceKey := range system.Resources {
			if resourceKey.String() == dependency.String() {
				// already done, skip
				goto DONE
			}
		}

		for taskKey2, task2 := range system.Tasks {
			for _, product := range task2.Produces {
				if product.String() == dependency.String() {
					log.Printf("building %q to get %v for %q\n", taskKey2, dependency, taskKey)
					if _, err := Build(taskKey2, system); err != nil {
						return nil, fmt.Errorf("build %q for %q to get %v: %w", taskKey2, taskKey, dependency, err)
					}
					goto DONE
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

	log.Println("building", taskKey, "...")
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

type AKey int

func (a AKey) String() string {
	return fmt.Sprintf("A[%d]", a)
}

type BKey int

func (b BKey) String() string {
	return fmt.Sprintf("B[%d]", b)
}

type CKey [2]int

func (c CKey) String() string {
	return fmt.Sprintf("C[%d %d]", c[0], c[1])
}

func main() {
	example := System{
		Tasks:     map[string]Task{},
		Resources: map[ResourceKey]Resource{},
	}
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
					Produces:     []ResourceKey{CKey{i, j}},
					Action: func([]Resource) ([]Resource, error) {
						return []Resource{IntResource(i)}, nil
					},
				}
			case i == 0:
				task = Task{
					Description:  Option[string]{},
					Dependencies: nil,
					Produces:     []ResourceKey{CKey{i, j}},
					Action: func([]Resource) ([]Resource, error) {
						return []Resource{IntResource(j)}, nil
					},
				}
			default:
				task = Task{
					Description: Option[string]{},
					Dependencies: []ResourceKey{
						AKey(i),
						BKey(j),

						CKey{i - 1, j - 1}, // replace
						CKey{i, j - 1},     // insert
						CKey{i - 1, j},     // delete
					},
					Produces: []ResourceKey{CKey{i, j}},
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
