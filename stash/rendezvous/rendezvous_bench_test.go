package rendezvous

import (
	"fmt"
	"testing"
)

func generateNodes(count int) []hashableString {
	nodes := make([]hashableString, count)
	for i := 0; i < count; i++ {
		nodes[i] = hashableString(fmt.Sprintf("https://server%d.datacenter-us-west.example.com:8080/api/v1/storage", i))
	}
	return nodes
}

var largeKeys = []string{
	"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	"sha256:d4735e3a265e16eee03f59718b9b5d03019c07d8b6c51f90da3a666eec13ab35",
	"sha256:4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce",
	"/user/data/projects/large-monorepo/src/components/dashboard/analytics/charts/LineChart.tsx",
	"/user/data/projects/large-monorepo/src/components/dashboard/analytics/charts/BarChart.tsx",
}

func BenchmarkHashGet_10_LargeNodes(b *testing.B) {
	nodes := generateNodes(10)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.Get(largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashGet_50_LargeNodes(b *testing.B) {
	nodes := generateNodes(50)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.Get(largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashGet_100_LargeNodes(b *testing.B) {
	nodes := generateNodes(100)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.Get(largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashGetN_10_LargeNodes(b *testing.B) {
	nodes := generateNodes(10)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.GetN(3, largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashGetN_50_LargeNodes(b *testing.B) {
	nodes := generateNodes(50)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.GetN(3, largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashGetN_100_LargeNodes(b *testing.B) {
	nodes := generateNodes(100)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.GetN(5, largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashAdd(b *testing.B) {
	nodes := generateNodes(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash := New[hashableString]()
		hash.Add(nodes...)
	}
}

func BenchmarkHashRemove(b *testing.B) {
	nodes := generateNodes(10)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.Remove(nodes[i%len(nodes)])
		hash.Add(nodes[i%len(nodes)])
	}
}

func BenchmarkHashGetWithSecret(b *testing.B) {
	nodes := generateNodes(100)
	hash := NewWithSecret(0x1234567890ABCDEF, nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.Get(largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashGetN1_100_LargeNodes(b *testing.B) {
	nodes := generateNodes(100)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.GetN(1, largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashGetN3_100_LargeNodes(b *testing.B) {
	nodes := generateNodes(100)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.GetN(3, largeKeys[i%len(largeKeys)])
	}
}

func BenchmarkHashGetN10_100_LargeNodes(b *testing.B) {
	nodes := generateNodes(100)
	hash := New(nodes...)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.GetN(10, largeKeys[i%len(largeKeys)])
	}
}
