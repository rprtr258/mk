package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
)

const _cacheFilename = ".cache.json"

type Key struct {
	A, B string
	I, J int
}

type Cache[K comparable, V any] struct {
	Cache map[K]V
}

type cacheItem[K comparable, V any] struct {
	K K
	V V
}

type cacheItems[K comparable, V any] []cacheItem[K, V]

func loadCache(filename string) (Cache[Key, int], error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return Cache[Key, int]{Cache: map[Key]int{}}, nil
		}

		return Cache[Key, int]{}, fmt.Errorf("read cache file: %w", err)
	}

	var items cacheItems[Key, int]
	if err := json.Unmarshal(b, &items); err != nil {
		return Cache[Key, int]{}, fmt.Errorf("json unmarshal: %w", err)
	}

	return Cache[Key, int]{
		Cache: fun.ToMap(items, func(elem cacheItem[Key, int]) (Key, int) {
			return elem.K, elem.V
		}),
	}, nil
}

func saveCache(filename string, cache Cache[Key, int]) error {
	b, err := json.Marshal(fun.ToSlice(cache.Cache, func(k Key, v int) cacheItem[Key, int] {
		return cacheItem[Key, int]{
			K: k,
			V: v,
		}
	}))
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	return os.WriteFile(filename, b, 0o644)
}

func editDistanceHelper(a, b string, i, j int, cache Cache[Key, int]) int {
	key := Key{a, b, i, j}

	if res, ok := cache.Cache[key]; ok {
		return res
	}

	switch {
	case j == 0:
		cache.Cache[key] = i
	case i == 0:
		cache.Cache[key] = j
	default:
		ac := a[i-1]
		bc := b[j-1]
		replace := editDistanceHelper(a, b, i-1, j-1, cache)
		if ac == bc {
			cache.Cache[key] = replace
		} else {
			insert := editDistanceHelper(a, b, i, j-1, cache) + 1
			delete := editDistanceHelper(a, b, i-1, j, cache) + 1

			cache.Cache[key] = fun.Min(replace+1, insert, delete)
		}
	}

	return cache.Cache[key]
}

func editDistance(a, b string, cache Cache[Key, int]) int {
	return editDistanceHelper(a, b, len(a), len(b), cache)
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
					if err := os.Remove(_cacheFilename); err != nil {
						if os.IsNotExist(err) {
							return nil
						}

						return fmt.Errorf("rm cachefile: %w", err)
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

					// TODO: get cache filename as ".%q_%q.editcache.json" % (a, b)
					cache, err := loadCache(_cacheFilename)
					if err != nil {
						log.Warnf("invalid cache file, try running prune command to reset it", log.F{"err": err.Error()})
						cache = Cache[Key, int]{Cache: map[Key]int{}}
					}

					distance := editDistance(a, b, cache)

					log.Infof("distance found", log.F{"distance": distance})

					if err := saveCache(_cacheFilename, cache); err != nil {
						return fmt.Errorf("save cache: %w", err)
					}

					return nil
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
