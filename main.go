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
	Tasks map[string]Task
	// TODO: separate by resource type?
	Resources map[ResourceKey]Resource
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
					Description: Option[string]{},
					Produces:    []ResourceKey{CKey{i, j}},
					Action: func(Fetcher) ([]Resource, error) {
						return []Resource{IntResource(i)}, nil
					},
				}
			case i == 0:
				task = Task{
					Description: Option[string]{},
					Produces:    []ResourceKey{CKey{i, j}},
					Action: func(Fetcher) ([]Resource, error) {
						return []Resource{IntResource(j)}, nil
					},
				}
			default:
				task = Task{
					Description: Option[string]{},
					Produces:    []ResourceKey{CKey{i, j}},
					Action: func(fetch Fetcher) ([]Resource, error) {
						defer log.Println("building", CKey{i, j}, "...")
						ac, err := fetch.String(AKey(i))
						if err != nil {
							return nil, err
						}
						bc, err := fetch.String(BKey(j))
						if err != nil {
							return nil, err
						}
						replace, err := fetch.Int(CKey{i - 1, j - 1})
						if err != nil {
							return nil, err
						}
						if ac == bc {
							return []Resource{IntResource(replace)}, nil
						}

						insert, err := fetch.Int(CKey{i, j - 1})
						if err != nil {
							return nil, err
						}
						delete, err := fetch.Int(CKey{i - 1, j})
						if err != nil {
							return nil, err
						}

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

	resources, err := example.Build("c5 5")
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println("output resources:")
	for _, v := range resources {
		fmt.Printf("\t%v\n", v)
	}

	fmt.Println("all resources:")
	for k, v := range example.Resources {
		fmt.Printf("\t%7s: %v\n", k.String(), v)
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
