package cache

import "errors"

// ErrContentNotFound is returned when content is not found in the cache.
var ErrContentNotFound = errors.New("content not found in cache")
