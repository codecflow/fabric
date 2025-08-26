package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/bsm/redislock"
	"github.com/codecflow/fabric/stash/metadata"
	"github.com/redis/go-redis/v9"
)

// Backend implements metadata.Backend using Redis.
type Backend struct {
	client redis.UniversalClient
	locker *redislock.Client
	locks  map[string]*redislock.Lock
	lockMu sync.Mutex
}

// New creates a new Redis backend from configuration.
func New(cfg Config) (metadata.Backend, error) {
	var client redis.UniversalClient

	switch cfg.Mode {
	case ModeSingle:
		opts := &redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
		}
		if cfg.TLSEnabled {
			opts.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		}
		client = redis.NewClient(opts)

	case ModeCluster:
		opts := &redis.ClusterOptions{
			Addrs:    []string{cfg.Addr},
			Password: cfg.Password,
		}
		if cfg.TLSEnabled {
			opts.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		}
		client = redis.NewClusterClient(opts)

	case ModeSentinel:
		opts := &redis.FailoverOptions{
			MasterName:       cfg.MasterName,
			SentinelAddrs:    []string{cfg.Addr},
			Password:         cfg.Password,
			SentinelPassword: cfg.Password,
			RouteByLatency:   true,
		}
		if cfg.TLSEnabled {
			opts.TLSConfig = &tls.Config{InsecureSkipVerify: true}
		}
		client = redis.NewFailoverClient(opts)

	default:
		return nil, fmt.Errorf("unsupported redis mode: %s", cfg.Mode)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &Backend{
		client: client,
		locker: redislock.New(client),
		locks:  make(map[string]*redislock.Lock),
	}, nil
}

// SAdd adds members to a set.
func (b *Backend) SAdd(ctx context.Context, key string, members ...string) error {
	return b.client.SAdd(ctx, key, members).Err()
}

// SRem removes members from a set.
func (b *Backend) SRem(ctx context.Context, key string, members ...string) error {
	return b.client.SRem(ctx, key, members).Err()
}

// SMembers returns all members of a set.
func (b *Backend) SMembers(ctx context.Context, key string) ([]string, error) {
	return b.client.SMembers(ctx, key).Result()
}

// Set stores a key-value pair with optional TTL.
func (b *Backend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return b.client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a value by key.
// Returns metadata.ErrKeyNotFound if the key doesn't exist.
func (b *Backend) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := b.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, metadata.ErrKeyNotFound
	}
	return result, err
}

// Del deletes a key.
func (b *Backend) Del(ctx context.Context, key string) error {
	return b.client.Del(ctx, key).Err()
}

// HSet stores multiple fields in a hash.
func (b *Backend) HSet(ctx context.Context, key string, fields map[string]interface{}) error {
	return b.client.HSet(ctx, key, fields).Err()
}

// HGetAll retrieves all fields from a hash.
func (b *Backend) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return b.client.HGetAll(ctx, key).Result()
}

// Lock acquires a distributed lock.
// Returns an unlock function that should be called to release the lock.
func (b *Backend) Lock(ctx context.Context, key string, ttl time.Duration, retries int) (func() error, error) {
	b.lockMu.Lock()
	defer b.lockMu.Unlock()

	var retryStrategy redislock.RetryStrategy
	if retries > 0 {
		retryStrategy = redislock.LimitRetry(
			redislock.LinearBackoff(100*time.Millisecond),
			retries,
		)
	}

	lock, err := b.locker.Obtain(ctx, key, ttl, &redislock.Options{
		RetryStrategy: retryStrategy,
	})
	if err != nil {
		return nil, err
	}

	// Store the lock for potential refresh
	b.locks[key] = lock

	// Return unlock function
	unlock := func() error {
		b.lockMu.Lock()
		defer b.lockMu.Unlock()

		if err := lock.Release(context.Background()); err != nil {
			return err
		}
		delete(b.locks, key)
		return nil
	}

	return unlock, nil
}

// RefreshLock extends the TTL of an existing lock.
func (b *Backend) RefreshLock(ctx context.Context, key string, ttl time.Duration) error {
	b.lockMu.Lock()
	defer b.lockMu.Unlock()

	lock, exists := b.locks[key]
	if !exists {
		return fmt.Errorf("lock not found: %s", key)
	}

	return lock.Refresh(ctx, ttl, nil)
}

// Close releases all locks and closes the Redis connection.
func (b *Backend) Close() error {
	b.lockMu.Lock()
	defer b.lockMu.Unlock()

	// Release all held locks
	for _, lock := range b.locks {
		lock.Release(context.Background())
	}
	b.locks = make(map[string]*redislock.Lock)

	return b.client.Close()
}
