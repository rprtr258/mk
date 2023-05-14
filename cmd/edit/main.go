package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
	"github.com/rprtr258/mk/cache"
)

type Key struct {
	I, J int
}

func editDistanceHelper(a, b string, i, j int, cache cache.Cache[Key, int]) int {
	key := Key{i, j}

	if res, ok := cache[key]; ok {
		return res
	}

	switch {
	case j == 0:
		cache[key] = i
	case i == 0:
		cache[key] = j
	default:
		ac := a[i-1]
		bc := b[j-1]
		replace := editDistanceHelper(a, b, i-1, j-1, cache)
		if ac == bc {
			cache[key] = replace
		} else {
			insert := editDistanceHelper(a, b, i, j-1, cache) + 1
			delete := editDistanceHelper(a, b, i-1, j, cache) + 1

			cache[key] = fun.Min(replace+1, insert, delete)
		}
	}

	return cache[key]
}

func editDistance(a, b string, cache cache.Cache[Key, int]) int {
	return editDistanceHelper(a, b, len(a), len(b), cache)
}

func getCacheFilename(a, b string) string {
	return fmt.Sprintf(".%q_%q.editcache.json", a, b)
}

func main() {
	if err := (&cli.App{
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

					for _, file := range files {
						if err := os.Remove(file); err != nil {
							if os.IsNotExist(err) {
								return nil
							}

							return fmt.Errorf("rm cachefile %q: %w", file, err)
						}
					}

					return nil
				},
			},
			{
				Name:      "dist",
				Usage:     "Calculate edit distance between two strings",
				UsageText: "edit dist <first> <second>",
				Action: func(ctx *cli.Context) error {
					if ctx.Args().Len() != 2 {
						return fmt.Errorf("expected 2 arguments, found %d", ctx.Args().Len())

					}

					args := ctx.Args().Slice()
					a, b := args[0], args[1]

					cacheFilename := getCacheFilename(a, b)

					_cache := cache.Load[Key, int](cacheFilename)

					distance := editDistance(a, b, _cache)

					log.Infof("distance found", log.F{"distance": distance})

					cache.Save(cacheFilename, _cache)

					return nil
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
