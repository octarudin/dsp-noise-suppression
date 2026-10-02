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

func TestSuppressVehicleNoiseReducesCalibratedEngineNoise(t *testing.T) {
	const rate = 48000
	x := make([]float64, rate*3)
	for i := range x {
		time := float64(i) / rate
		engine := 0.18*math.Sin(2*math.Pi*110*time) +
			0.11*math.Sin(2*math.Pi*220*time) +
			0.07*math.Sin(2*math.Pi*440*time) +
			0.04*math.Sin(2*math.Pi*880*time)
		x[i] = engine
		if i >= rate*2 {
			x[i] += 0.12 * math.Sin(2*math.Pi*1000*time)
		}
	}
	y := SuppressVehicleNoise(x, rate, rate)
	if len(y) != len(x) {
		t.Fatalf("length %d, ingin %d", len(y), len(x))
	}
	inputRMS := signalRMS(x[rate : rate*2])
	outputRMS := signalRMS(y[rate : rate*2])
	if outputRMS >= inputRMS*0.5 {
		t.Fatalf("noise kurang teredam: input %.5f output %.5f", inputRMS, outputRMS)
	}
	voiceAmplitude := toneAmplitude(y[rate*2:rate*3], rate, 1000)
	if voiceAmplitude < 0.04 {
		t.Fatalf("komponen suara terlalu teredam: amplitudo %.5f", voiceAmplitude)
	}
	for _, value := range y {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatal("output vehicle suppression tidak finite")
		}
	}
}

func signalRMS(x []float64) float64 {
	var sum float64
	for _, value := range x {
		sum += value * value
	}
	return math.Sqrt(sum / float64(len(x)))
}

func toneAmplitude(x []float64, rate, frequency int) float64 {
	var sine, cosine float64
	for i, value := range x {
		angle := 2 * math.Pi * float64(frequency*i) / float64(rate)
		sine += value * math.Sin(angle)
		cosine += value * math.Cos(angle)
	}
	return 2 * math.Hypot(sine, cosine) / float64(len(x))
}
func cmplxAbs(x complex128) float64 { return math.Hypot(real(x), imag(x)) }
