package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const sampleRate = 48000

func main() {
	duration := flag.Duration("duration", 10*time.Second, "durasi rekaman")
	deviceIndex := flag.Int("device", -1, "indeks perangkat input (-1 menggunakan default)")
	outDir := flag.String("output", ".", "direktori output")
	listDevices := flag.Bool("list-devices", false, "tampilkan perangkat input lalu keluar")
	flag.Parse()

	if *listDevices {
		if err := printCaptureDevices(); err != nil {
			fatal(err)
		}
		return
	}
	if *duration <= 0 {
		fatal(fmt.Errorf("duration harus lebih besar dari nol"))
	}
	stamp := time.Now().Format("2006-01-02-15-04-05")
	sessionDir := filepath.Join(*outDir, stamp)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		fatal(fmt.Errorf("membuat direktori sesi: %w", err))
	}
	metricsPath := filepath.Join(sessionDir, "metrics-"+stamp+".csv")
	monitor, err := startMetricsMonitor(metricsPath, 250*time.Millisecond)
	if err != nil {
		fatal(fmt.Errorf("memulai pencatatan metrik: %w", err))
	}

	fmt.Printf("Merekam mono 48 kHz selama %s...\n", duration.String())
	raw, deviceName, captureErr := captureAudio(*duration, *deviceIndex)
	summary, metricsErr := monitor.Stop()
	if captureErr != nil {
		fatal(captureErr)
	}
	if metricsErr != nil {
		fatal(fmt.Errorf("menyimpan metrik: %w", metricsErr))
	}

	rawPath := filepath.Join(sessionDir, "raw-"+stamp+".wav")
	if err := writeWAV(rawPath, raw, sampleRate); err != nil {
		fatal(err)
	}

	fmt.Println("Memproses tiga metode DSP klasik...")
	methods := []struct {
		name string
		kind SuppressionMethod
	}{
		{"omlsa-imcra", MethodOMLSA},
		{"logmmse", MethodLogMMSE},
		{"wiener", MethodWiener},
	}
	for _, method := range methods {
		filtered := SuppressNoise(raw, sampleRate, method.kind)
		path := filepath.Join(sessionDir, "filtered-"+method.name+"-"+stamp+".wav")
		if err := writeWAV(path, filtered, sampleRate); err != nil {
			fatal(err)
		}
		fmt.Printf("  %s\n", path)
	}

	fmt.Printf("\nPerangkat : %s\n", deviceName)
	fmt.Printf("Folder sesi: %s\n", sessionDir)
	fmt.Printf("Audio mentah: %s\n", rawPath)
	fmt.Printf("Metrik      : %s\n", metricsPath)
	fmt.Printf("CPU         : rata-rata %.2f%%, puncak %.2f%%\n", summary.AvgCPU, summary.PeakCPU)
	fmt.Printf("RAM         : rata-rata %.2f MiB, puncak %.2f MiB\n", summary.AvgRAMMiB, summary.PeakRAMMiB)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
