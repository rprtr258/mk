package cache

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rprtr258/fun"
	"github.com/rprtr258/log"
)

type Cache[K comparable, V any] map[K]V

type cacheItem[K comparable, V any] struct {
	K K
	V V
}

type cacheItems[K comparable, V any] []cacheItem[K, V]

func Load[K comparable, V any](filename string) Cache[K, V] {
	b, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return map[K]V{}
		}

		log.Warnf("invalid cache file, try running prune command to reset it", log.F{"err": fmt.Errorf("read cache file: %w", err).Error()})
		return map[K]V{}
	}

	var items cacheItems[K, V]
	if err := json.Unmarshal(b, &items); err != nil {
		log.Warnf("invalid cache file, try running prune command to reset it", log.F{"err": fmt.Errorf("json unmarshal: %w", err).Error()})
		return map[K]V{}
	}

	return fun.ToMap(items, func(elem cacheItem[K, V]) (K, V) {
		return elem.K, elem.V
	})
}

func Save[K comparable, V any](filename string, cache Cache[K, V]) {
	b, err := json.Marshal(fun.ToSlice(cache, func(k K, v V) cacheItem[K, V] {
		return cacheItem[K, V]{
			K: k,
			V: v,
		}
	}))
	if err != nil {
		log.Warnf("save cache failed", log.F{"err": fmt.Errorf("json marshal: %w", err)})
	}

	if err := os.WriteFile(filename, b, 0o644); err != nil {
		log.Warnf("save cache failed", log.F{"err": fmt.Errorf("write file %q: %w", filename, err)})
	}
}
