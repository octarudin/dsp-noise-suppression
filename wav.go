package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

func writeWAV(path string, samples []float64, rate int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("membuat %s: %w", path, err)
	}
	w := bufio.NewWriter(f)
	dataSize := uint32(len(samples) * 2)
	write := func(v any) error { return binary.Write(w, binary.LittleEndian, v) }
	if _, err = w.WriteString("RIFF"); err == nil {
		err = write(uint32(36) + dataSize)
	}
	if err == nil {
		_, err = w.WriteString("WAVEfmt ")
	}
	if err == nil {
		err = write(uint32(16))
	}
	if err == nil {
		err = write(uint16(1))
	}
	if err == nil {
		err = write(uint16(1))
	}
	if err == nil {
		err = write(uint32(rate))
	}
	if err == nil {
		err = write(uint32(rate * 2))
	}
	if err == nil {
		err = write(uint16(2))
	}
	if err == nil {
		err = write(uint16(16))
	}
	if err == nil {
		_, err = w.WriteString("data")
	}
	if err == nil {
		err = write(dataSize)
	}
	for _, sample := range samples {
		if err != nil {
			break
		}
		sample = math.Max(-1, math.Min(1, sample))
		var value int16
		if sample < 0 {
			value = int16(math.Round(sample * 32768))
		} else {
			value = int16(math.Round(sample * 32767))
		}
		err = write(value)
	}
	if flushErr := w.Flush(); err == nil {
		err = flushErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("menulis %s: %w", path, err)
	}
	return nil
}
