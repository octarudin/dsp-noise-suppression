//go:build !windows

package main

import (
	"runtime"
	"time"
)

// Fallback portabel: RAM adalah heap Go. Implementasi Windows melaporkan working set proses.
type processProbe struct{}

func newProcessProbe() (*processProbe, error) { return &processProbe{}, nil }
func (*processProbe) sample() (float64, uint64, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	_ = time.Now()
	return 0, m.Sys, nil
}
