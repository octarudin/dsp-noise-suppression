//go:build linux

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type alsaDevice struct {
	card, device int
	name, id     string
}

var alsaDevicePattern = regexp.MustCompile(`(?m)^card ([0-9]+): (.+), device ([0-9]+): (.+)$`)

func alsaCaptureDevices() ([]alsaDevice, error) {
	if _, err := exec.LookPath("arecord"); err != nil {
		return nil, fmt.Errorf("arecord tidak ditemukan; instal paket alsa-utils: %w", err)
	}
	cmd := exec.Command("arecord", "-l")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("membaca perangkat ALSA: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	matches := alsaDevicePattern.FindAllStringSubmatch(strings.ReplaceAll(string(output), "\r\n", "\n"), -1)
	devices := make([]alsaDevice, 0, len(matches))
	for _, match := range matches {
		card, cardErr := strconv.Atoi(match[1])
		device, deviceErr := strconv.Atoi(match[3])
		if cardErr != nil || deviceErr != nil {
			continue
		}
		devices = append(devices, alsaDevice{
			card: card, device: device,
			name: strings.TrimSpace(match[2]) + " / " + strings.TrimSpace(match[4]),
			id:   fmt.Sprintf("plughw:%d,%d", card, device),
		})
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("tidak ada perangkat capture ALSA; periksa koneksi mikrofon dan jalankan arecord -l")
	}
	return devices, nil
}

func printCaptureDevices() error {
	devices, err := alsaCaptureDevices()
	if err != nil {
		return err
	}
	for i, device := range devices {
		fmt.Printf("  %d: %s (%s)\n", i, device.name, device.id)
	}
	fmt.Println("Gunakan tanpa --device untuk input default ALSA.")
	return nil
}

func captureAudio(duration time.Duration, deviceIndex int) ([]float64, string, error) {
	if _, err := exec.LookPath("arecord"); err != nil {
		return nil, "", fmt.Errorf("arecord tidak ditemukan; instal paket alsa-utils: %w", err)
	}
	deviceID, deviceName := "default", "input default ALSA"
	if deviceIndex >= 0 {
		devices, err := alsaCaptureDevices()
		if err != nil {
			return nil, "", err
		}
		if deviceIndex >= len(devices) {
			return nil, "", fmt.Errorf("indeks perangkat %d tidak tersedia; gunakan --list-devices", deviceIndex)
		}
		deviceID, deviceName = devices[deviceIndex].id, devices[deviceIndex].name
	}

	targetSamples := int(math.Round(duration.Seconds() * sampleRate))
	raw := make([]byte, targetSamples*2)
	cmd := exec.Command(
		"arecord", "-q", "-D", deviceID, "-t", "raw", "-f", "S16_LE",
		"-r", strconv.Itoa(sampleRate), "-c", "1", "-",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("membuka output arecord: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("memulai arecord: %w", err)
	}

	_, readErr := io.ReadFull(stdout, raw)
	// Tepat setelah jumlah sampel target diterima, hentikan arecord. Error Wait
	// akibat SIGINT bukan kegagalan karena seluruh audio sudah tersedia.
	_ = cmd.Process.Signal(syscall.SIGINT)
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		_ = cmd.Process.Kill()
		<-waitDone
	}
	if readErr != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = readErr.Error()
		}
		return nil, "", fmt.Errorf("perekaman ALSA berhenti sebelum %s: %s", duration, detail)
	}

	result := make([]float64, targetSamples)
	for i := range result {
		value := int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
		result[i] = float64(value) / 32768
	}
	return result, deviceName, nil
}
