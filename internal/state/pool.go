package state

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// PoolKey hashes an ordered backend name list so round-robin state is disjoint per pool.
func PoolKey(orderedBackendNames []string) string {
	if len(orderedBackendNames) == 0 {
		return ""
	}
	h := sha256.Sum256([]byte(strings.Join(orderedBackendNames, "\x00")))
	return hex.EncodeToString(h[:12])
}
