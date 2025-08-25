package rendezvous

import (
	"cmp"
	"slices"
	"sync"
)

type score struct {
	index int
	score uint64
	hash  uint64
}

type scores struct {
	items []score
}

var scoresPool = sync.Pool{
	New: func() any {
		return &scores{
			items: make([]score, 0, 128),
		}
	},
}

func acquire(size int) *scores {
	s := scoresPool.Get().(*scores)
	if cap(s.items) < size {
		s.items = make([]score, size)
	} else {
		s.items = s.items[:size]
	}
	return s
}

func (s *scores) release() {
	s.items = s.items[:0]
	scoresPool.Put(s)
}

func (s *scores) sort() {
	slices.SortFunc(s.items, func(a, b score) int {
		if a.score != b.score {
			return cmp.Compare(b.score, a.score)
		}
		return cmp.Compare(a.hash, b.hash)
	})
}
