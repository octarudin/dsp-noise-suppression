//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const linuxClockTicksPerSecond = 100

type linuxProcessStats struct {
	pid      int
	ticks    uint64
	ramBytes uint64
}

type processProbe struct {
	rootPID   int
	lastTicks map[int]uint64
	lastWall  time.Time
}

func newProcessProbe() (*processProbe, error) {
	pid := os.Getpid()
	stat, err := readLinuxProcessStats(pid)
	if err != nil {
		return nil, err
	}
	return &processProbe{
		rootPID: pid, lastTicks: map[int]uint64{pid: stat.ticks}, lastWall: time.Now(),
	}, nil
}

func (p *processProbe) sample() (float64, uint64, error) {
	stats, err := p.processTreeStats()
	if err != nil {
		return 0, 0, err
	}
	now := time.Now()
	var deltaTicks, ram uint64
	nextTicks := make(map[int]uint64, len(stats))
	for _, stat := range stats {
		previous, existed := p.lastTicks[stat.pid]
		if existed && stat.ticks >= previous {
			deltaTicks += stat.ticks - previous
		} else if !existed {
			// Proses anak lahir setelah sampel sebelumnya, sehingga seluruh tick-nya
			// merupakan penggunaan dalam interval pengukuran.
			deltaTicks += stat.ticks
		}
		nextTicks[stat.pid] = stat.ticks
		ram += stat.ramBytes
	}
	elapsed := now.Sub(p.lastWall).Seconds()
	cpu := float64(deltaTicks) / linuxClockTicksPerSecond / elapsed / float64(runtime.NumCPU()) * 100
	p.lastTicks, p.lastWall = nextTicks, now
	return cpu, ram, nil
}

func (p *processProbe) processTreeStats() ([]linuxProcessStats, error) {
	root, err := readLinuxProcessStats(p.rootPID)
	if err != nil {
		return nil, err
	}
	stats := []linuxProcessStats{root}
	paths, _ := filepath.Glob(fmt.Sprintf("/proc/%d/task/*/children", p.rootPID))
	children := make(map[int]struct{})
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(data)) {
			pid, err := strconv.Atoi(field)
			if err == nil {
				children[pid] = struct{}{}
			}
		}
	}
	for pid := range children {
		stat, err := readLinuxProcessStats(pid)
		if err == nil {
			stats = append(stats, stat)
		}
	}
	return stats, nil
}

func readLinuxProcessStats(pid int) (linuxProcessStats, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return linuxProcessStats{}, err
	}
	// Nama proses di field kedua dapat mengandung spasi dan tanda kurung buka.
	// Tanda kurung tutup terakhir menandai akhir nama tersebut.
	endName := strings.LastIndexByte(string(data), ')')
	if endName < 0 {
		return linuxProcessStats{}, fmt.Errorf("format /proc/%d/stat tidak valid", pid)
	}
	fields := strings.Fields(string(data[endName+1:]))
	if len(fields) < 13 {
		return linuxProcessStats{}, fmt.Errorf("field /proc/%d/stat tidak lengkap", pid)
	}
	userTicks, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return linuxProcessStats{}, err
	}
	systemTicks, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return linuxProcessStats{}, err
	}
	ramBytes, err := linuxResidentMemory(pid)
	if err != nil {
		return linuxProcessStats{}, err
	}
	return linuxProcessStats{pid: pid, ticks: userTicks + systemTicks, ramBytes: ramBytes}, nil
}

func linuxResidentMemory(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}
		return kib * 1024, nil
	}
	return 0, fmt.Errorf("VmRSS tidak ditemukan untuk PID %d", pid)
}
