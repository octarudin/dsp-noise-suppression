package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// readWAV membaca WAV RIFF PCM 16-bit mono dan mengubah sampelnya ke rentang
// [-1, 1]. Chunk metadata tambahan dilewati sehingga file dari recorder umum
// tetap dapat dibaca.
func readWAV(path string) ([]float64, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("membuka %s: %w", path, err)
	}
	defer f.Close()

	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return nil, 0, fmt.Errorf("membaca header WAV %s: %w", path, err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("%s bukan file WAV RIFF", path)
	}

	var formatFound bool
	var rate uint32
	for {
		var chunkHeader [8]byte
		if _, err := io.ReadFull(f, chunkHeader[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, 0, fmt.Errorf("membaca chunk WAV %s: %w", path, err)
		}
		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, 0, fmt.Errorf("%s memiliki chunk fmt WAV yang tidak valid", path)
			}
			var format [16]byte
			if _, err := io.ReadFull(f, format[:]); err != nil {
				return nil, 0, fmt.Errorf("membaca format WAV %s: %w", path, err)
			}
			audioFormat := binary.LittleEndian.Uint16(format[0:2])
			channels := binary.LittleEndian.Uint16(format[2:4])
			rate = binary.LittleEndian.Uint32(format[4:8])
			bitsPerSample := binary.LittleEndian.Uint16(format[14:16])
			if audioFormat != 1 || channels != 1 || bitsPerSample != 16 || rate == 0 {
				return nil, 0, fmt.Errorf("%s harus berupa WAV PCM 16-bit mono (format=%d, kanal=%d, bit=%d, sample-rate=%d)", path, audioFormat, channels, bitsPerSample, rate)
			}
			if _, err := io.CopyN(io.Discard, f, int64(chunkSize)-16); err != nil {
				return nil, 0, fmt.Errorf("membaca format WAV %s: %w", path, err)
			}
			formatFound = true
		case "data":
			if !formatFound {
				return nil, 0, fmt.Errorf("%s memiliki chunk data sebelum chunk fmt", path)
			}
			if chunkSize%2 != 0 {
				return nil, 0, fmt.Errorf("%s memiliki ukuran data PCM 16-bit yang tidak valid", path)
			}
			samples := make([]float64, int(chunkSize/2))
			var sampleBytes [2]byte
			for i := range samples {
				if _, err := io.ReadFull(f, sampleBytes[:]); err != nil {
					return nil, 0, fmt.Errorf("membaca sampel WAV %s: %w", path, err)
				}
				value := int16(binary.LittleEndian.Uint16(sampleBytes[:]))
				if value < 0 {
					samples[i] = float64(value) / 32768
				} else {
					samples[i] = float64(value) / 32767
				}
			}
			return samples, int(rate), nil
		default:
			if _, err := io.CopyN(io.Discard, f, int64(chunkSize)); err != nil {
				return nil, 0, fmt.Errorf("melewati chunk WAV %s: %w", path, err)
			}
		}
		if chunkSize%2 != 0 {
			if _, err := io.CopyN(io.Discard, f, 1); err != nil {
				return nil, 0, fmt.Errorf("membaca padding chunk WAV %s: %w", path, err)
			}
		}
	}
	return nil, 0, fmt.Errorf("%s tidak memiliki chunk data audio", path)
}

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
