package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/rprtr258/fun"
	"github.com/rs/zerolog/log"
)

// TODO: add thread safety
type Cache[K comparable, V any] map[K]V

// CompareAndSwap value by key k to newV, returns true if value was changed
func CompareAndSwap[K, V comparable](cache Cache[K, V], k K, newV V) bool {
	oldV, ok := cache[k]
	if !ok || oldV != newV {
		cache[k] = newV
		return true
	}

	return false
}

func (c Cache[K, V]) GetOrEval(key K, eval func() V) V {
	if _, ok := c[key]; !ok {
		c[key] = eval()
	}

	return c[key]
}

type cacheItem[K comparable, V any] struct {
	K K
	V V
}

type cacheItems[K comparable, V any] []cacheItem[K, V]

func LoadFromFile[K comparable, V any](filename string) Cache[K, V] {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return map[K]V{}
		}

		log.Warn().Str("filename", filename).Err(err).Msg("can't open cache file")
		return map[K]V{}
	}
	defer file.Close()

	return Load[K, V](file)
}

func Load[K comparable, V any](r io.Reader) Cache[K, V] {
	var items cacheItems[K, V]
	if err := json.NewDecoder(r).Decode(&items); err != nil {
		log.Warn().Err(err).Msg("invalid cache file, try running prune command to reset it")
		return map[K]V{}
	}

	return fun.SliceToMap[K, V](func(elem cacheItem[K, V]) (K, V) {
		return elem.K, elem.V
	}, items...)
}

func SaveToFile[K comparable, V any](filename string, cache Cache[K, V]) {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		log.Warn().Str("filename", filename).Err(err).Msg("can't open cache file")
		return
	}
	defer file.Close()

	Save(file, cache)
}

func Save[K comparable, V any](w io.Writer, cache Cache[K, V]) {
	if err := json.NewEncoder(w).Encode(fun.MapToSlice(cache, func(k K, v V) cacheItem[K, V] {
		return cacheItem[K, V]{
			K: k,
			V: v,
		}
	})); err != nil {
		log.Warn().Err(err).Msg("save cache failed")
	}
}

// TODO: move out?
func HashFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, errCopy := io.Copy(h, f); errCopy != nil {
		return "", fmt.Errorf("sha256 hashing: %w", errCopy)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashDir / HashGlob

func WithCache[K comparable, V any](filename string, f func(Cache[K, V]) error) error {
	cache := LoadFromFile[K, V](filename)

	if err := f(cache); err != nil {
		return err
	}

	SaveToFile(filename, cache)

	return nil
}
