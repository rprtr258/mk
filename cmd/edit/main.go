package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"

	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
	"github.com/rprtr258/mk/cache"
)

type Key struct {
	I, J int
}

type runner struct {
	cache cache.Cache[Key, int]
}

func (k runner) distance(a, b string, i, j int) int {
	return k.cache.GetOrEval(Key{i, j}, func() int {
		switch {
		case j == 0:
			return i
		case i == 0:
			return j
		default:
			replaceCost := k.distance(a, b, i-1, j-1)
			if a[i-1] == b[j-1] {
				return replaceCost
			}

			insertCost := k.distance(a, b, i, j-1) + 1
			deleteCost := k.distance(a, b, i-1, j) + 1

			return fun.Min(replaceCost+1, insertCost, deleteCost)
		}
	})
}

func getCacheFilename(a, b string) string {
	return fmt.Sprintf(".%q_%q.editcache.json", a, b)
}

func main() {
	if err := (&cli.App{ //nolint:exhaustruct // daaaaa
		Name:  "edit",
		Usage: "edit distance runner",
		Commands: []*cli.Command{
			{
				Name:  "prune",
				Usage: "remove cache",
				Action: func(*cli.Context) error {
					files, err := filepath.Glob(".*_*.editcache.json")
					if err != nil {
						return fmt.Errorf("glob: %w", err)
					}

					var merr error
					for _, file := range files {
						if errRm := os.Remove(file); errRm != nil {
							if os.IsNotExist(errRm) {
								continue
							}

							multierr.AppendInto(&merr, fmt.Errorf("rm cachefile %q: %w", file, errRm))
						}
					}
					return merr
				},
			},
			{
				Name:      "dist",
				Usage:     "Calculate edit distance between two strings",
				UsageText: "edit dist <first> <second>",
				Action: func(ctx *cli.Context) error {
					if ctx.Args().Len() != 2 { //nolint:gomnd // 2 string: a and b are expected
						return fmt.Errorf("expected 2 arguments, found %d", ctx.Args().Len())
					}

					args := ctx.Args().Slice()
					a, b := args[0], args[1]

					return cache.WithCache( //nolint:wrapcheck // daaaaaa
						getCacheFilename(a, b),
						func(c cache.Cache[Key, int]) error {
							distance := runner{c}.distance(a, b, len(a), len(b))

							log.Infof("distance found", log.F{"distance": distance})

							return nil
						},
					)
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
