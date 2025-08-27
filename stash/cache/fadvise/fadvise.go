//go:build linux

package fadvise

import (
	"golang.org/x/sys/unix"
)

// Sequential hints that the file will be read sequentially.
// This triggers more aggressive kernel readahead.
func Sequential(fd uintptr) error {
	return unix.Fadvise(int(fd), 0, 0, unix.FADV_SEQUENTIAL)
}

// Willneed hints that data will be accessed soon.
// This can trigger asynchronous prefetch into page cache.
func Willneed(fd uintptr, offset, length int64) error {
	return unix.Fadvise(int(fd), offset, length, unix.FADV_WILLNEED)
}

// Dontneed hints that data won't be needed anymore.
// Useful for streaming workloads to avoid cache pollution.
func Dontneed(fd uintptr, offset, length int64) error {
	return unix.Fadvise(int(fd), offset, length, unix.FADV_DONTNEED)
}

// Random hints that file will be accessed randomly.
// This disables readahead to avoid wasting I/O bandwidth.
func Random(fd uintptr) error {
	return unix.Fadvise(int(fd), 0, 0, unix.FADV_RANDOM)
}
