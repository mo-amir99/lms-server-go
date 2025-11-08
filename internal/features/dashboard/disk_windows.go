//go:build windows
// +build windows

package dashboard

import (
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// getDiskStatsForPlatform gets disk stats for Windows
func getDiskStatsForPlatform(path string) *DiskStats {
	var freeBytesAvailable, totalBytes, totalFreeBytes int64

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return &DiskStats{Free: 0, Size: 0, Path: path}
	}

	ret, _, _ := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		// Failed to get disk space
		return &DiskStats{Free: 0, Size: 0, Path: path}
	}

	return &DiskStats{
		Free: uint64(freeBytesAvailable),
		Size: uint64(totalBytes),
		Path: path,
	}
}
