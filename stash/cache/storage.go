package cache

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/cache/fadvise"
	"github.com/codecflow/fabric/stash/config"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/shirou/gopsutil/v4/mem"
)

const (
	diskCacheUsageCheckInterval = 1 * time.Minute
)

// Storage is a content-addressable storage with two-tier caching (memory + disk).
type Storage struct {
	ctx                     context.Context
	currentHost             *stash.Host
	locality                string
	cache                   *ristretto.Cache[string, cacheValue]
	serverConfig            config.Server
	globalConfig            config.Global
	maxCacheSizeMb          int64
	diskCacheDir            string
	diskCachedUsageExceeded bool
	mu                      sync.Mutex
	metrics                 *Metrics
	bufferPool              *BufferPool
	prefetcher              *Prefetcher
	logger                  *slog.Logger
}

type cacheValue struct {
	Hash    string
	Content []byte
}

// NewStorage creates a new content-addressable storage instance.
func NewStorage(ctx context.Context, currentHost *stash.Host, locality string, cfg *config.BlobCacheConfig, logger *slog.Logger) (*Storage, error) {
	if cfg.Server.PageSizeBytes <= 0 {
		return nil, fmt.Errorf("invalid cache configuration: page size must be positive")
	}

	if logger == nil {
		logger = slog.Default()
	}

	s := &Storage{
		ctx:          ctx,
		serverConfig: cfg.Server,
		globalConfig: cfg.Global,
		currentHost:  currentHost,
		locality:     locality,
		diskCacheDir: cfg.Server.DiskCacheDir,
		logger:       logger,
		metrics:      NewMetrics(),
		bufferPool:   NewBufferPool(),
	}

	logger.Info("disk cache directory", "path", s.diskCacheDir)

	// Memory cache enabled by default if MaxCachePct > 0
	enableMemCache := cfg.Server.MaxCachePct > 0

	if enableMemCache {
		_, totalMemoryMb := getMemoryMb()
		maxCacheSizeMb := (totalMemoryMb * cfg.Server.MaxCachePct) / 100
		maxCost := maxCacheSizeMb * 1e6

		logger.Info("memory cache enabled",
			"total_memory_mb", totalMemoryMb,
			"max_cache_size_mb", maxCacheSizeMb,
			"max_cost", maxCost)

		if maxCacheSizeMb <= 0 {
			return nil, fmt.Errorf("invalid memory limit: %d MB", maxCacheSizeMb)
		}

		cache, err := ristretto.NewCache(&ristretto.Config[string, cacheValue]{
			NumCounters: 1e7,
			MaxCost:     maxCost,
			BufferItems: 64,
			OnEvict:     s.onEvict,
			Metrics:     cfg.Global.DebugMode,
		})
		if err != nil {
			return nil, err
		}

		s.cache = cache
		s.maxCacheSizeMb = maxCacheSizeMb
	} else {
		logger.Info("memory cache disabled (disk-only mode)")
		// Create minimal cache for metadata tracking
		cache, _ := ristretto.NewCache(&ristretto.Config[string, cacheValue]{
			NumCounters: 1e5,
			MaxCost:     1024 * 1024, // 1MB for metadata only
			BufferItems: 64,
			OnEvict:     s.onEvict,
			Metrics:     false,
		})
		s.cache = cache
	}

	// Initialize prefetcher
	s.prefetcher = NewPrefetcher(ctx, s, s.bufferPool, logger)

	// Start disk usage monitor
	go s.monitorDiskCacheUsage()

	return s, nil
}

// Add stores content in the cache with the given hash.
func (s *Storage) Add(ctx context.Context, hash string, content []byte) error {
	size := int64(len(content))
	chunkKeys := []string{}

	dirPath := filepath.Join(s.diskCacheDir, hash)
	if !s.diskCachedUsageExceeded {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	// Break content into chunks and store
	for offset := int64(0); offset < size; offset += s.serverConfig.PageSizeBytes {
		chunkIdx := offset / s.serverConfig.PageSizeBytes
		end := offset + s.serverConfig.PageSizeBytes
		if end > size {
			end = size
		}

		// Copy the chunk into a new buffer
		chunk := make([]byte, end-offset)
		copy(chunk, content[offset:end])
		chunkKey := fmt.Sprintf("%s-%d", hash, chunkIdx)

		// Write through to disk cache if we still have storage available
		if !s.diskCachedUsageExceeded {
			filePath := filepath.Join(dirPath, chunkKey)
			if err := os.WriteFile(filePath, chunk, 0644); err != nil {
				return fmt.Errorf("failed to write to disk cache: %w", err)
			}
		}

		chunkKeys = append(chunkKeys, chunkKey)

		_, exists := s.cache.Get(chunkKey)
		if exists {
			continue
		}

		// Store the chunk
		added := s.cache.Set(chunkKey, cacheValue{Hash: hash, Content: chunk}, int64(len(chunk)))
		if !added {
			return fmt.Errorf("unable to cache: set dropped")
		}
	}

	// Release the large initial buffer
	content = nil

	// Store chunk keys in cache
	chunks := strings.Join(chunkKeys, ",")
	ttl := time.Duration(s.serverConfig.ObjectTtlS) * time.Second
	added := s.cache.SetWithTTL(hash, cacheValue{Hash: hash, Content: []byte(chunks)}, int64(len(chunks)), ttl)
	if !added {
		return fmt.Errorf("unable to cache: set dropped")
	}

	s.logger.Debug("added object", "hash", hash, "size", size)
	return nil
}

// Exists checks if content with the given hash is in the cache.
func (s *Storage) Exists(hash string) bool {
	_, exists := s.cache.Get(hash)
	if !exists {
		// Check disk
		stat, err := os.Stat(filepath.Join(s.diskCacheDir, hash))
		if err != nil {
			return false
		}
		return stat.IsDir()
	}
	return exists
}

// Get retrieves content from the cache.
func (s *Storage) Get(hash string, offset, length int64, dst []byte) (int64, error) {
	remainingLength := length
	o := offset
	dstOffset := int64(0)

	// Track metrics
	s.metrics.TotalReads.Inc()
	start := time.Now()
	defer func() {
		if dstOffset > 0 {
			throughputMBps := (float64(dstOffset) / (1024 * 1024)) / (float64(time.Since(start).Microseconds()) / 1e6)
			s.metrics.ReadThroughputMBps.Update(throughputMBps)
		}
		s.updateHitRatios()
	}()

	// Notify prefetcher about this read
	s.prefetcher.OnRead(hash, offset, length)

	// Reset TTL
	ttl := time.Duration(s.serverConfig.ObjectTtlS) * time.Second
	s.cache.Get(hash) // This resets TTL in ristretto

	for remainingLength > 0 {
		chunkIdx := o / s.serverConfig.PageSizeBytes
		chunkKey := fmt.Sprintf("%s-%d", hash, chunkIdx)

		// Check in-memory cache for chunk (L0)
		value, found := s.cache.Get(chunkKey)
		fromMemory := found
		if found {
			s.metrics.L0Hits.Inc()
		}

		// Not found in memory, check disk cache (L1)
		if !found {
			var err error
			value, found, err = s.getFromDiskCache(hash, chunkKey, ttl)
			if err != nil || !found {
				s.metrics.L2Misses.Inc()
				return 0, ErrContentNotFound
			}
			s.metrics.L1Hits.Inc()
		}

		chunkBytes := value.Content
		start := o % s.serverConfig.PageSizeBytes
		chunkRemaining := int64(len(chunkBytes)) - start
		if chunkRemaining <= 0 {
			break
		}

		readLength := min(remainingLength, chunkRemaining)
		end := start + readLength

		if start < 0 || end <= start || end > int64(len(chunkBytes)) {
			return 0, fmt.Errorf("invalid chunk boundaries: start %d, end %d, chunk size %d", start, end, len(chunkBytes))
		}

		copy(dst[dstOffset:dstOffset+readLength], chunkBytes[start:end])

		// Track bytes served from appropriate tier
		if fromMemory {
			s.metrics.L0BytesServed.Add(int(readLength))
		} else {
			s.metrics.L1BytesServed.Add(int(readLength))
		}

		remainingLength -= readLength
		o += readLength
		dstOffset += readLength
	}

	return dstOffset, nil
}

// getFromDiskCache reads a chunk from disk and loads it into memory.
func (s *Storage) getFromDiskCache(hash, chunkKey string, ttl time.Duration) (value cacheValue, found bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check cache
	rawValue, found := s.cache.Get(chunkKey)
	if found {
		return rawValue, true, nil
	}

	chunkPath := filepath.Join(s.diskCacheDir, hash, chunkKey)

	// Use fadvise to hint sequential/random access patterns
	file, err := os.Open(chunkPath)
	if err != nil {
		return cacheValue{}, false, ErrContentNotFound
	}
	defer file.Close()

	// Hint sequential access for better readahead
	if err := fadvise.Sequential(file.Fd()); err == nil {
		s.logger.Debug("set FADV_SEQUENTIAL", "path", chunkPath)
	}

	chunkBytes, err := os.ReadFile(chunkPath)
	if err != nil {
		return cacheValue{}, false, ErrContentNotFound
	}

	// Hint willneed for prefetch
	if err := fadvise.Willneed(file.Fd(), 0, int64(len(chunkBytes))); err == nil {
		s.logger.Debug("set FADV_WILLNEED", "path", chunkPath)
	}

	value = cacheValue{Hash: hash, Content: chunkBytes}
	s.cache.SetWithTTL(chunkKey, value, int64(len(chunkBytes)), ttl)

	return value, true, nil
}

// onEvict is called when an item is evicted from the cache.
func (s *Storage) onEvict(item *ristretto.Item[cacheValue]) {
	hash := ""
	var chunkKeys []string

	// Extract hash and chunk keys
	v := item.Value
	if strings.Contains(string(v.Content), ",") {
		// This is the chunk list
		hash = strings.SplitN(string(v.Content), "-", 2)[0]
		chunkKeys = strings.Split(string(v.Content), ",")
	} else {
		// This is a chunk
		hash = v.Hash
		chunks, found := s.cache.Get(hash)
		if found {
			chunkKeys = strings.Split(string(chunks.Content), ",")
		}
	}

	s.logger.Info("evicted object", "hash", hash)
	s.logger.Debug("evicted object chunks", "chunks", chunkKeys)

	for _, k := range chunkKeys {
		s.cache.Del(k)
	}
}

// Cleanup closes the cache and releases resources.
func (s *Storage) Cleanup() {
	s.cache.Close()
}

// GetDiskCacheMetrics returns disk cache usage statistics.
func (s *Storage) GetDiskCacheMetrics() (int64, int64, float64, error) {
	diskUsageMb, err := getDiskUsageMb(s.diskCacheDir)
	if err != nil {
		return 0, 0, 0, err
	}

	totalDiskSpaceMb, err := getTotalDiskSpaceMb(s.diskCacheDir)
	if err != nil {
		return 0, 0, 0, err
	}

	usagePercentage := float64(0)
	if totalDiskSpaceMb > 0 {
		usagePercentage = (float64(diskUsageMb) / float64(totalDiskSpaceMb)) * 100
	}

	return diskUsageMb, totalDiskSpaceMb, usagePercentage, nil
}

// monitorDiskCacheUsage periodically monitors disk and memory usage.
func (s *Storage) monitorDiskCacheUsage() {
	ticker := time.NewTicker(diskCacheUsageCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			currentUsage, totalDiskSpace, usagePercentage, err := s.GetDiskCacheMetrics()
			if err != nil {
				if !os.IsNotExist(err) {
					s.logger.Error("failed to fetch disk cache metrics", "error", err)
				}
				continue
			}

			availableMemoryMb, totalMemoryMb := getMemoryMb()
			usedMemoryMb := totalMemoryMb - availableMemoryMb
			s.metrics.MemCacheUsageMB.Update(float64(usedMemoryMb))
			s.metrics.MemCacheUsagePct.Update(float64(usedMemoryMb) / float64(totalMemoryMb) * 100)
			s.metrics.DiskCacheUsageMB.Update(float64(currentUsage))
			s.metrics.DiskCacheUsagePct.Update(usagePercentage)

			s.logger.Info("cache usage",
				"mem_mb", usedMemoryMb,
				"mem_total_mb", totalMemoryMb,
				"disk_mb", currentUsage,
				"disk_total_mb", totalDiskSpace)

			// Update internal state for disk usage exceeded
			s.mu.Lock()
			s.diskCachedUsageExceeded = usagePercentage > s.serverConfig.DiskCacheMaxUsagePct
			s.mu.Unlock()
		}
	}
}

// updateHitRatios calculates and updates cache hit ratio metrics.
func (s *Storage) updateHitRatios() {
	l0Hits := s.metrics.L0Hits.Get()
	l1Hits := s.metrics.L1Hits.Get()
	l2Misses := s.metrics.L2Misses.Get()
	total := l0Hits + l1Hits + l2Misses

	if total > 0 {
		l0Ratio := float64(l0Hits) / float64(total) * 100
		l1Ratio := float64(l1Hits) / float64(total) * 100
		l2Ratio := float64(l2Misses) / float64(total) * 100

		s.metrics.L0HitRatio.Update(l0Ratio)
		s.metrics.L1HitRatio.Update(l1Ratio)
		s.metrics.L2MissRatio.Update(l2Ratio)
	}
}

// Helper functions

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func getDiskUsageMb(path string) (int64, error) {
	var totalUsage int64
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalUsage += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return totalUsage / (1024 * 1024), nil
}

func getTotalDiskSpaceMb(path string) (int64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	return int64(stat.Blocks) * int64(stat.Bsize) / (1024 * 1024), nil
}

func getMemoryMb() (int64, int64) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0
	}
	return int64(v.Available / (1024 * 1024)), int64(v.Total / (1024 * 1024))
}
