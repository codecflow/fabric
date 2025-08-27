//go:build !linux

package fadvise

// Sequential is a no-op on non-Linux platforms.
func Sequential(fd uintptr) error {
	return nil
}

// Willneed is a no-op on non-Linux platforms.
func Willneed(fd uintptr, offset, length int64) error {
	return nil
}

// Dontneed is a no-op on non-Linux platforms.
func Dontneed(fd uintptr, offset, length int64) error {
	return nil
}

// Random is a no-op on non-Linux platforms.
func Random(fd uintptr) error {
	return nil
}
