package rush

import (
	"fmt"
	"hash/fnv"
	"math/rand"
)

func seededPRNGRank(seed int64, parts ...interface{}) uint64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "seed=%d", seed)
	for _, part := range parts {
		_, _ = fmt.Fprintf(h, "|%v", part)
	}
	rng := rand.New(rand.NewSource(int64(h.Sum64())))
	return rng.Uint64()
}
