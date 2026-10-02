package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

type metricSample struct {
	elapsed  time.Duration
	cpu      float64
	ramBytes uint64
}
type MetricsSummary struct{ AvgCPU, PeakCPU, AvgRAMMiB, PeakRAMMiB float64 }
type metricsMonitor struct {
	stop    chan struct{}
	done    chan struct{}
	samples []metricSample
	path    string
	err     error
}

func startMetricsMonitor(path string, interval time.Duration) (*metricsMonitor, error) {
	probe, err := newProcessProbe()
	if err != nil {
		return nil, err
	}
	m := &metricsMonitor{stop: make(chan struct{}), done: make(chan struct{}), path: path}
	start := time.Now()
	go func() {
		defer close(m.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cpu, ram, err := probe.sample()
				if err != nil {
					m.err = err
					return
				}
				m.samples = append(m.samples, metricSample{time.Since(start), cpu, ram})
			case <-m.stop:
				return
			}
		}
	}()
	return m, nil
}

func (m *metricsMonitor) Stop() (MetricsSummary, error) {
	close(m.stop)
	<-m.done
	if m.err != nil {
		return MetricsSummary{}, m.err
	}
	f, err := os.Create(m.path)
	if err != nil {
		return MetricsSummary{}, err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"elapsed-ms", "cpu-percent", "ram-mib"})
	var s MetricsSummary
	for _, v := range m.samples {
		ram := float64(v.ramBytes) / (1024 * 1024)
		_ = w.Write([]string{strconv.FormatInt(v.elapsed.Milliseconds(), 10), fmt.Sprintf("%.4f", v.cpu), fmt.Sprintf("%.4f", ram)})
		s.AvgCPU += v.cpu
		s.AvgRAMMiB += ram
		if v.cpu > s.PeakCPU {
			s.PeakCPU = v.cpu
		}
		if ram > s.PeakRAMMiB {
			s.PeakRAMMiB = ram
		}
	}
	if len(m.samples) > 0 {
		s.AvgCPU /= float64(len(m.samples))
		s.AvgRAMMiB /= float64(len(m.samples))
	}
	w.Flush()
	if err = w.Error(); err == nil {
		err = f.Close()
	} else {
		_ = f.Close()
	}
	return s, err
}
