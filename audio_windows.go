//go:build windows

package main

import (
	"fmt"
	"math"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	waveFormatPCM = 1
	waveMapper    = 0xffffffff
	whdrDone      = 0x00000001
)

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	ExtraSize      uint16
}

type waveHeader struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

type waveInCaps struct {
	ManufacturerID uint16
	ProductID      uint16
	DriverVersion  uint32
	ProductName    [32]uint16
	Formats        uint32
	Channels       uint16
	Reserved       uint16
}

var (
	winmm                 = syscall.NewLazyDLL("winmm.dll")
	waveInGetNumDevs      = winmm.NewProc("waveInGetNumDevs")
	waveInGetDevCapsW     = winmm.NewProc("waveInGetDevCapsW")
	waveInOpen            = winmm.NewProc("waveInOpen")
	waveInPrepareHeader   = winmm.NewProc("waveInPrepareHeader")
	waveInUnprepareHeader = winmm.NewProc("waveInUnprepareHeader")
	waveInAddBuffer       = winmm.NewProc("waveInAddBuffer")
	waveInStart           = winmm.NewProc("waveInStart")
	waveInStop            = winmm.NewProc("waveInStop")
	waveInReset           = winmm.NewProc("waveInReset")
	waveInClose           = winmm.NewProc("waveInClose")
)

func printCaptureDevices() error {
	count, _, _ := waveInGetNumDevs.Call()
	if count == 0 {
		fmt.Println("Tidak ada perangkat input audio.")
		return nil
	}
	for i := uintptr(0); i < count; i++ {
		name, err := captureDeviceName(uint32(i))
		if err != nil {
			return err
		}
		fmt.Printf("  %d: %s\n", i, name)
	}
	fmt.Println("Gunakan tanpa --device untuk input default Windows.")
	return nil
}

func captureDeviceName(index uint32) (string, error) {
	var caps waveInCaps
	code, _, _ := waveInGetDevCapsW.Call(uintptr(index), uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps))
	if code != 0 {
		return "", fmt.Errorf("waveInGetDevCaps(%d) gagal, kode MMRESULT %d", index, code)
	}
	end := 0
	for end < len(caps.ProductName) && caps.ProductName[end] != 0 {
		end++
	}
	return string(utf16.Decode(caps.ProductName[:end])), nil
}

func captureAudio(duration time.Duration, deviceIndex int) ([]float64, string, error) {
	count, _, _ := waveInGetNumDevs.Call()
	if deviceIndex >= 0 && uintptr(deviceIndex) >= count {
		return nil, "", fmt.Errorf("indeks perangkat %d tidak tersedia; gunakan --list-devices", deviceIndex)
	}
	deviceID := uintptr(waveMapper)
	deviceName := "input default Windows"
	if deviceIndex >= 0 {
		deviceID = uintptr(deviceIndex)
		var err error
		deviceName, err = captureDeviceName(uint32(deviceIndex))
		if err != nil {
			return nil, "", err
		}
	}

	format := waveFormatEx{
		FormatTag: waveFormatPCM, Channels: 1, SamplesPerSec: sampleRate,
		AvgBytesPerSec: sampleRate * 2, BlockAlign: 2, BitsPerSample: 16,
	}
	var handle uintptr
	code, _, _ := waveInOpen.Call(
		uintptr(unsafe.Pointer(&handle)), deviceID, uintptr(unsafe.Pointer(&format)), 0, 0, 0,
	)
	if code != 0 {
		return nil, "", fmt.Errorf("membuka input 48 kHz mono gagal, kode MMRESULT %d", code)
	}
	defer waveInClose.Call(handle)

	targetSamples := int(math.Round(duration.Seconds() * sampleRate))
	buffer := make([]int16, targetSamples)
	header := waveHeader{Data: uintptr(unsafe.Pointer(&buffer[0])), BufferLength: uint32(len(buffer) * 2)}
	headerSize := unsafe.Sizeof(header)
	code, _, _ = waveInPrepareHeader.Call(handle, uintptr(unsafe.Pointer(&header)), headerSize)
	if code != 0 {
		return nil, "", fmt.Errorf("menyiapkan buffer audio gagal, kode MMRESULT %d", code)
	}
	defer func() {
		waveInReset.Call(handle)
		waveInUnprepareHeader.Call(handle, uintptr(unsafe.Pointer(&header)), headerSize)
	}()
	code, _, _ = waveInAddBuffer.Call(handle, uintptr(unsafe.Pointer(&header)), headerSize)
	if code != 0 {
		return nil, "", fmt.Errorf("mengirim buffer audio gagal, kode MMRESULT %d", code)
	}
	code, _, _ = waveInStart.Call(handle)
	if code != 0 {
		return nil, "", fmt.Errorf("memulai perekaman gagal, kode MMRESULT %d", code)
	}

	deadline := time.Now().Add(duration + 2*time.Second)
	for atomic.LoadUint32(&header.Flags)&whdrDone == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	waveInStop.Call(handle)
	if atomic.LoadUint32(&header.Flags)&whdrDone == 0 {
		return nil, "", fmt.Errorf("perekaman timeout; driver merekam %d byte", atomic.LoadUint32(&header.BytesRecorded))
	}
	recorded := int(atomic.LoadUint32(&header.BytesRecorded) / 2)
	if recorded > len(buffer) {
		recorded = len(buffer)
	}
	result := make([]float64, recorded)
	for i, value := range buffer[:recorded] {
		result[i] = float64(value) / 32768
	}
	return result, deviceName, nil
}
