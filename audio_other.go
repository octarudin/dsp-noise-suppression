//go:build !windows

package main

import (
	"fmt"
	"time"
)

func printCaptureDevices() error {
	return fmt.Errorf("perekaman audio saat ini diimplementasikan untuk Windows")
}

func captureAudio(time.Duration, int) ([]float64, string, error) {
	return nil, "", fmt.Errorf("perekaman audio saat ini diimplementasikan untuk Windows")
}
