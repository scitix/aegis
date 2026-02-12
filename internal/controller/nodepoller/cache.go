package nodepoller

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"time"

	"github.com/scitix/aegis/pkg/prom"
)

// criticalEntry tracks a node that is currently in the critical set.
type criticalEntry struct {
	alertName    string // name of the AegisAlert created for this node
	lastStatuses []prom.AegisNodeStatus
	version      int64
	since        time.Time
}

// statusVersion computes a stable int64 version hash for a slice of AegisNodeStatus.
func statusVersion(statuses []prom.AegisNodeStatus) int64 {
	// Build a deterministic string from sorted (condition, id) pairs
	type pair struct{ cond, id string }
	pairs := make([]pair, 0, len(statuses))
	for _, s := range statuses {
		pairs = append(pairs, pair{s.Condition, s.ID})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].cond != pairs[j].cond {
			return pairs[i].cond < pairs[j].cond
		}
		return pairs[i].id < pairs[j].id
	})

	var b []byte
	for _, p := range pairs {
		b = append(b, []byte(p.cond+"\x00"+p.id+"\x01")...)
	}
	h := sha256.Sum256(b)
	return int64(binary.BigEndian.Uint64(h[:8]))
}
