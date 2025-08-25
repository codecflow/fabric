// Package rendezvous implements rendezvous hashing (a.k.a. highest random weight hashing).
package rendezvous

import (
	"encoding/binary"
	"slices"
	"sync"

	"github.com/zeebo/xxh3"
)

// Hashable nodes must provide stable byte representation.
type Hashable interface {
	Bytes() []byte
}

type entry[N Hashable] struct {
	node      N
	bytesHash uint64
}

// Hash implements rendezvous hashing over nodes of type N.
type Hash[N Hashable] struct {
	mu     sync.RWMutex
	nodes  []entry[N]
	secret uint64 // Optional secret for preventing key steering attacks
}

// New creates a Hash with the given nodes.
func New[N Hashable](nodes ...N) *Hash[N] {
	h := &Hash[N]{}
	h.Add(nodes...)
	return h
}

// NewWithSecret creates a Hash with a secret for key steering protection.
func NewWithSecret[N Hashable](secret uint64, nodes ...N) *Hash[N] {
	h := &Hash[N]{secret: secret}
	h.Add(nodes...)
	return h
}

// Add adds nodes to the hash.
func (h *Hash[N]) Add(nodes ...N) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, n := range nodes {
		bytesHash := xxh3.Hash(n.Bytes())
		h.nodes = append(h.nodes, entry[N]{node: n, bytesHash: bytesHash})
	}
}

// Remove removes nodes matching the provided node's hash.
func (h *Hash[N]) Remove(node N) {
	toRemoveHash := xxh3.Hash(node.Bytes())

	h.mu.Lock()
	defer h.mu.Unlock()

	h.nodes = slices.DeleteFunc(h.nodes, func(e entry[N]) bool {
		return e.bytesHash == toRemoveHash
	})
}

// Get returns the node with the highest score for key.
// If no nodes exist, returns (zero, false).
func (h *Hash[N]) Get(key string) (N, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.nodes) == 0 {
		var zero N
		return zero, false
	}

	seed := xxh3.HashString(key) ^ h.secret

	best := h.nodes[0]
	bestScore := hashWithSeed(best.bytesHash, seed)

	for i := 1; i < len(h.nodes); i++ {
		e := h.nodes[i]
		s := hashWithSeed(e.bytesHash, seed)

		// Highest score wins; tie-break by bytesHash for determinism.
		if s > bestScore || (s == bestScore && e.bytesHash < best.bytesHash) {
			best = e
			bestScore = s
		}
	}

	return best.node, true
}

// GetN returns up to n nodes for key, ordered by descending score.
// This does NOT mutate or reorder the internal node list.
// Optimized with hash-only storage, fast uint64 comparisons, and managed pooling.
func (h *Hash[N]) GetN(n int, key string) []N {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.nodes) == 0 || n <= 0 {
		return nil
	}
	if n > len(h.nodes) {
		n = len(h.nodes)
	}

	seed := xxh3.HashString(key) ^ h.secret

	// Acquire pooled indexed scores
	indices := acquire(len(h.nodes))
	defer indices.release()

	// Populate scores
	for i, e := range h.nodes {
		indices.items[i] = score{
			index: i,
			score: hashWithSeed(e.bytesHash, seed),
			hash:  e.bytesHash,
		}
	}

	// Sort by score
	indices.sort()

	// Dereference top N indices
	out := make([]N, n)
	for i := 0; i < n; i++ {
		out[i] = h.nodes[indices.items[i].index].node
	}

	return out
}

// hashWithSeed hashes a uint64 with a seed for proper avalanche mixing
func hashWithSeed(hash, seed uint64) uint64 {
	// Convert uint64 to bytes and hash with seed
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], hash)
	return xxh3.HashSeed(b[:], seed)
}
