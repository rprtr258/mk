package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rprtr258/log"
	"github.com/rprtr258/mk"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
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

func newSystem(a, b string) mk.System {
	system := mk.System{
		Resources: map[mk.ResourceKey]mk.Resource{},
		Tasks:     map[string]mk.Task{},
	}
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
						defer log.Infof("building", log.F{"task": CKey{i, j}})
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
	if err := (&cli.App{
		Name:  "edit",
		Usage: "edit distance runner",
		Commands: []*cli.Command{
			{
				Name: "dist",
				// Usage: "<first> <second>",
				Usage: "Calculate edit distance between two strings",
				Action: func(ctx *cli.Context) error {
					if ctx.Args().Len() != 2 {
						return fmt.Errorf("expected 2 arguments, found %d", ctx.Args().Len())

					}

					args := ctx.Args().Slice()
					a, b := args[0], args[1]
					system := newSystem(a, b)

					resources, err := system.Build(fmt.Sprintf("c%d %d", len(a)-1, len(b)-1))
					if err != nil {
						return err
					}

					fmt.Println("output resources:", strings.Join(lo.Map(resources, func(v mk.Resource, _ int) string {
						return fmt.Sprintf("\t%v\n", v)
					}), " "))

					fmt.Println("all resources:")
					for k, v := range system.Resources {
						fmt.Printf("\t%7s: %v\n", k.String(), v)
					}

					return nil
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
