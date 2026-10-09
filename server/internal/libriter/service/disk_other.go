//go:build !unix

package service

// diskUsage na systémech bez Statfs místo nezjišťuje – administrace pak
// políčko s volným místem vynechá.
func diskUsage(string) *DiskUsage { return nil }
