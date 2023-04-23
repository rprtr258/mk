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
	docstring    Option[string]
	dependencies []Resource
	produces     []Resource
	action       string
}

type Tasks = map[string]Task

func build(t string, tasks Tasks) {
	task, ok := tasks[t]
	if !ok {
		fmt.Printf("%q task was not found\n", t)
		return
	}

	for _, kk := range task.dependencies {
		switch tt := kk.(type) {
		case TaskResource:
			build(string(tt), tasks)
		}
	}
	fmt.Println(task.action)
}

func main() {
	example := Tasks{
		"compile": {docstring: Option[string]{"build executable", true}, dependencies: []Resource{FileResource("main.go")}, produces: []Resource{FileResource("mk")}, action: "go build -o mk main.go"},
		"run":     {docstring: Option[string]{"run main", true}, dependencies: []Resource{TaskResource("compile")}, produces: nil, action: "./mk"},
	}

	fmt.Println("a::")
	build("a", example)

	fmt.Println("\ncompile::")
	build("compile", example)

	fmt.Println("\nrun::")
	build("run", example)
}
