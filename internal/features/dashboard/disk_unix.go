//go:build linux || darwin
// +build linux darwin

package dashboard

import "syscall"

// getDiskStatsForPlatform gets disk stats for Unix-like systems
func getDiskStatsForPlatform(path string) *DiskStats {
	var stat syscall.Statfs_t

	if err := syscall.Statfs(path, &stat); err != nil {
		// Return placeholder on error
		return &DiskStats{
			Free: 0,
			Size: 0,
			Path: path,
		}
	}

	// Calculate sizes (works for Linux, macOS, BSD)
	free := stat.Bavail * uint64(stat.Bsize)
	size := stat.Blocks * uint64(stat.Bsize)

	return &DiskStats{
		Free: free,
		Size: size,
		Path: path,
	}
}
