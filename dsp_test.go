package main

import (
	"math"
	"testing"
)

func TestFFTInverse(t *testing.T) {
	x := make([]complex128, 1024)
	for i := range x {
		x[i] = complex(math.Sin(2*math.Pi*17*float64(i)/1024), 0)
	}
	original := append([]complex128(nil), x...)
	fft(x, false)
	fft(x, true)
	for i := range x {
		if cmplxAbs(x[i]-original[i]) > 1e-9 {
			t.Fatalf("round trip error at %d", i)
		}
	}
}
func TestSuppressNoiseLengthAndFinite(t *testing.T) {
	x := make([]float64, 48000)
	for i := range x {
		x[i] = 0.2*math.Sin(2*math.Pi*440*float64(i)/48000) + 0.01*math.Sin(2*math.Pi*7000*float64(i)/48000)
	}
	for _, m := range []SuppressionMethod{MethodOMLSA, MethodLogMMSE, MethodWiener} {
		y := SuppressNoise(x, 48000, m)
		if len(y) != len(x) {
			t.Fatalf("length %d", len(y))
		}
		for _, v := range y {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatal("non-finite output")
			}
		}
	}
}
func cmplxAbs(x complex128) float64 { return math.Hypot(real(x), imag(x)) }
