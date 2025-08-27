package cache

import "sync"

// BufferPool manages reusable byte buffers to reduce allocations.
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Default 4MB buffers
				return make([]byte, 4*1024*1024)
			},
		},
	}
}

// Get retrieves a buffer from the pool of at least the specified size.
func (p *BufferPool) Get(size int) []byte {
	buf := p.pool.Get().([]byte)
	if len(buf) < size {
		// Buffer too small, allocate new one
		return make([]byte, size)
	}
	return buf[:size]
}

// Put returns a buffer to the pool for reuse.
func (p *BufferPool) Put(buf []byte) {
	if cap(buf) >= 4*1024*1024 { // Only pool buffers >= 4MB
		p.pool.Put(buf)
	}
}
