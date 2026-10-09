//go:build unix

package service

import "syscall"

// diskUsage zjistí volné místo svazku. Počítá se s blocky dostupnými běžnému
// uživateli (Bavail), ne s rezervou pro roota.
func diskUsage(path string) *DiskUsage {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil
	}

	blockSize := uint64(st.Bsize) //nolint:gosec // Bsize je vždy kladné
	return &DiskUsage{
		Path:       path,
		FreeBytes:  st.Bavail * blockSize,
		TotalBytes: st.Blocks * blockSize,
	}
}
