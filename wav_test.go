package main

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWAVRoundTrip(t *testing.T) {
	want := []float64{-1, -0.5, 0, 0.5, 1}
	path := filepath.Join(t.TempDir(), "round-trip.wav")
	if err := writeWAV(path, want, 48000); err != nil {
		t.Fatal(err)
	}
	got, rate, err := readWAV(path)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 48000 {
		t.Fatalf("sample rate = %d, ingin 48000", rate)
	}
	if len(got) != len(want) {
		t.Fatalf("jumlah sampel = %d, ingin %d", len(got), len(want))
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1.0/32767 {
			t.Fatalf("sampel %d = %f, ingin %f", i, got[i], want[i])
		}
	}
}

func TestReadWAVRejectsStereo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stereo.wav")
	data := make([]byte, 48)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:16], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 2)
	binary.LittleEndian.PutUint32(data[24:28], 48000)
	binary.LittleEndian.PutUint32(data[28:32], 192000)
	binary.LittleEndian.PutUint16(data[32:34], 4)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], 4)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := readWAV(path)
	if err == nil || !strings.Contains(err.Error(), "kanal=2") {
		t.Fatalf("ingin error stereo yang jelas, mendapat %v", err)
	}
}
