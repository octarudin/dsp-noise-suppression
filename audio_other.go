//go:build !windows && !linux

package main

import (
	"fmt"
	"time"
)

func printCaptureDevices() error {
	return fmt.Errorf("perekaman audio didukung pada Windows dan Linux")
}

func captureAudio(time.Duration, int) ([]float64, string, error) {
	return nil, "", fmt.Errorf("perekaman audio didukung pada Windows dan Linux")
}
