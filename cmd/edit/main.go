package main

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"encoding/gob"
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

func (k Key) MarshalText() ([]byte, error) {
	// NOTE: i wanted to just use json here to get humanly readable keys BUT
	// json.Marshal uses MarshalText method if it is implemented which leads to
	// infinite recursion

	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(k); err != nil {
		return nil, fmt.Errorf("gob encode key %#v: %w", k, err)
	}

	return []byte(base64.StdEncoding.EncodeToString(b.Bytes())), nil
}

func (k *Key) UnmarshalText(b []byte) error {
	if err := gob.NewDecoder(base64.NewDecoder(base64.StdEncoding, bytes.NewBuffer(b))).Decode(k); err != nil {
		return fmt.Errorf("gob decode key: %w", err)
	}

	return nil
}

type Cache[K interface {
	encoding.TextMarshaler
	comparable
}, KP interface {
	*K
	encoding.TextUnmarshaler
}] struct {
	Cache map[K]int
}

func loadCache(filename string) (Cache[Key, *Key], error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return Cache[Key, *Key]{Cache: map[Key]int{}}, nil
		}

		return Cache[Key, *Key]{}, fmt.Errorf("read cache file: %w", err)
	}

	var res Cache[Key, *Key]
	if err := json.Unmarshal(b, &res); err != nil {
		return Cache[Key, *Key]{}, fmt.Errorf("json unmarshal: %w", err)
	}

	return res, nil
}

func saveCache(filename string, cache Cache[Key, *Key]) error {
	b, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	return os.WriteFile(filename, b, 0o644)
}

func editDistanceHelper(a, b string, i, j int, cache Cache[Key, *Key]) int {
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

func editDistance(a, b string, cache Cache[Key, *Key]) int {
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

					cache, err := loadCache(_cacheFilename)
					if err != nil {
						log.Warnf("invalid cache file, try running prune command to reset it", log.F{"err": err.Error()})
						cache = Cache[Key, *Key]{Cache: map[Key]int{}}
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
