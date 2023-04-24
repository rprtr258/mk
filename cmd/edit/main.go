package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/rprtr258/mk"
	"github.com/samber/lo"
)

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

var system = newSystem()

func newSystem() mk.System {
	system := mk.System{
		Tasks:     map[string]mk.Task{},
		Resources: map[mk.ResourceKey]mk.Resource{},
	}
	a := "aaakek"
	b := "aaalel"
	for i, c := range a {
		system.Resources[AKey(i)] = mk.StringResource(string(c))
	}
	for i, c := range b {
		system.Resources[BKey(i)] = mk.StringResource(string(c))
	}
	for i := 0; i < len(a); i++ {
		i := i

		for j := 0; j < len(b); j++ {
			j := j

			var task mk.Task
			switch {
			case j == 0:
				task = mk.Task{
					Description: mk.Option[string]{},
					Produces:    []mk.ResourceKey{CKey{i, j}},
					Action: func(mk.Fetcher) ([]mk.Resource, error) {
						return []mk.Resource{mk.IntResource(i)}, nil
					},
				}
			case i == 0:
				task = mk.Task{
					Description: mk.Option[string]{},
					Produces:    []mk.ResourceKey{CKey{i, j}},
					Action: func(mk.Fetcher) ([]mk.Resource, error) {
						return []mk.Resource{mk.IntResource(j)}, nil
					},
				}
			default:
				task = mk.Task{
					Description: mk.Option[string]{},
					Produces:    []mk.ResourceKey{CKey{i, j}},
					Action: func(fetch mk.Fetcher) ([]mk.Resource, error) {
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
							return []mk.Resource{mk.IntResource(replace)}, nil
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

						return []mk.Resource{mk.IntResource(1 + x)}, nil
					},
				}
			}
			system.Tasks[fmt.Sprintf("c%d %d", i, j)] = task
		}
	}
	return system
}

func main() {
	// for k, v := range example.Tasks {
	// 	fmt.Println(k, v)
	// }
	// fmt.Println()

	resources, err := system.Build("c5 5")
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println("output resources:", strings.Join(lo.Map(resources, func(v mk.Resource, _ int) string {
		return fmt.Sprintf("\t%v\n", v)
	}), " "))

	fmt.Println("all resources:")
	for k, v := range system.Resources {
		fmt.Printf("\t%7s: %v\n", k.String(), v)
	}
}
