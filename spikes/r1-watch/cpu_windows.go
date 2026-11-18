//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// cpu_windows.go: process CPU time and working set via Win32.

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procTimes         = kernel32.NewProc("GetProcessTimes")
	getCurrentProcess = kernel32.NewProc("GetCurrentProcess")
	psapi             = syscall.NewLazyDLL("psapi.dll")
	procMemInfo       = psapi.NewProc("GetProcessMemoryInfo")
)

type filetime struct {
	Low, High uint32
}

type processMemoryCounters struct {
	cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// procCPU returns process kernel+user CPU time in nanoseconds.
func procCPU() int64 {
	h, _, _ := getCurrentProcess.Call()
	var create, exit, kern, user filetime
	r, _, _ := procTimes.Call(h, uintptr(unsafe.Pointer(&create)), uintptr(unsafe.Pointer(&exit)), uintptr(unsafe.Pointer(&kern)), uintptr(unsafe.Pointer(&user)))
	if r == 0 {
		return 0
	}
	k := int64(kern.High)<<32 | int64(kern.Low)
	u := int64(user.High)<<32 | int64(user.Low)
	// FILETIME is in 100ns units.
	return (k + u) * 100
}

// procRSS returns the process working set in bytes.
func procRSS() uint64 {
	h, _, _ := getCurrentProcess.Call()
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	r, _, _ := procMemInfo.Call(h, uintptr(unsafe.Pointer(&c)), uintptr(c.cb))
	if r == 0 {
		return 0
	}
	return uint64(c.WorkingSetSize)
}
