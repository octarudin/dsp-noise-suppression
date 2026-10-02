//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

type filetime struct{ LowDateTime, HighDateTime uint32 }
type processMemoryCounters struct {
	CB, PageFaultCount                                                                                 uint32
	PeakWorkingSetSize, WorkingSetSize, QuotaPeakPagedPoolUsage, QuotaPagedPoolUsage                   uintptr
	QuotaPeakNonPagedPoolUsage, QuotaNonPagedPoolUsage, PagefileUsage, PeakPagefileUsage, PrivateUsage uintptr
}
type processProbe struct {
	handle   uintptr
	lastCPU  uint64
	lastWall time.Time
}

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	psapi                = syscall.NewLazyDLL("psapi.dll")
	getCurrentProcess    = kernel32.NewProc("GetCurrentProcess")
	getProcessTimes      = kernel32.NewProc("GetProcessTimes")
	getProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
)

func newProcessProbe() (*processProbe, error) {
	h, _, _ := getCurrentProcess.Call()
	p := &processProbe{handle: h, lastWall: time.Now()}
	cpu, err := p.cpuTime()
	if err != nil {
		return nil, err
	}
	p.lastCPU = cpu
	return p, nil
}
func (p *processProbe) cpuTime() (uint64, error) {
	var creation, exit, kernel, user filetime
	r, _, e := getProcessTimes.Call(p.handle, uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if r == 0 {
		return 0, fmt.Errorf("GetProcessTimes: %v", e)
	}
	to64 := func(v filetime) uint64 { return uint64(v.HighDateTime)<<32 | uint64(v.LowDateTime) }
	return to64(kernel) + to64(user), nil
}
func (p *processProbe) sample() (float64, uint64, error) {
	now := time.Now()
	current, err := p.cpuTime()
	if err != nil {
		return 0, 0, err
	}
	elapsed := now.Sub(p.lastWall).Seconds()
	cpu := float64(current-p.lastCPU) / 1e7 / elapsed / float64(runtime.NumCPU()) * 100
	p.lastCPU = current
	p.lastWall = now
	mem := processMemoryCounters{}
	mem.CB = uint32(unsafe.Sizeof(mem))
	r, _, e := getProcessMemoryInfo.Call(p.handle, uintptr(unsafe.Pointer(&mem)), uintptr(mem.CB))
	if r == 0 {
		return 0, 0, fmt.Errorf("GetProcessMemoryInfo: %v", e)
	}
	return cpu, uint64(mem.WorkingSetSize), nil
}
