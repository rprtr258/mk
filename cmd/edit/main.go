package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"github.com/rprtr258/mk/cache"
)

type Key struct {
	I, J string
}

type runner struct {
	cache cache.Cache[Key, int]
}

func (k runner) distance(a, b []rune) int {
	return k.cache.GetOrEval(Key{string(a), string(b)}, func() int {
		switch {
		case len(b) == 0:
			return len(a)
		case len(a) == 0:
			return len(b)
		default:
			replace := k.distance(a[:len(a)-1], b[:len(b)-1])
			if a[len(a)-1] == b[len(b)-1] {
				return replace
			}

			return min(
				replace+1,
				k.distance(a, b[:len(b)-1])+1,
				k.distance(a[:len(a)-1], b)+1,
			)
		}
	})
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	const cacheFile = ".editcache.json"

	if err := (&cli.App{ //nolint:exhaustruct // daaaaa
		Name:  "edit",
		Usage: "edit distance runner",
		Commands: []*cli.Command{
			{
				Name:  "prune",
				Usage: "remove cache",
				Action: func(*cli.Context) error {
					if err := os.Remove(cacheFile); err != nil {
						if os.IsNotExist(err) {
							return nil
						}

						return fmt.Errorf("rm cachefile %q: %w", cacheFile, err)
					}

					return nil
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
						cacheFile,
						func(c cache.Cache[Key, int]) error {
							distance := runner{c}.distance([]rune(a), []rune(b))

							log.Info().Int("distance", distance).Msg("distance found")

							return nil
						},
					)
				},
			},
		},
	}).Run(os.Args); err != nil {
		log.Fatal().Err(err).Send()
	}
}
