package main

import (
	"math"
	"math/cmplx"
)

type SuppressionMethod int

const (
	MethodOMLSA SuppressionMethod = iota
	MethodLogMMSE
	MethodWiener
)

const (
	frameSize = 1024
	hopSize   = 256
	epsilon   = 1e-12
)

// SuppressNoise memakai analysis/synthesis STFT dengan Hann window dan 75% overlap.
// Ketiga metode berbagi estimator noise IMCRA agar perbandingannya adil.
func SuppressNoise(input []float64, rate int, method SuppressionMethod) []float64 {
	if len(input) == 0 {
		return nil
	}
	conditioned := precondition(input, float64(rate))
	pad := frameSize
	paddedLen := len(conditioned) + 2*pad
	frameCount := 1
	if paddedLen > frameSize {
		frameCount = 1 + (paddedLen-frameSize+hopSize-1)/hopSize
	}
	total := (frameCount-1)*hopSize + frameSize
	x := make([]float64, total)
	copy(x[pad:], conditioned)
	out, norm := make([]float64, total), make([]float64, total)
	window := make([]float64, frameSize)
	for i := range window {
		window[i] = math.Sqrt(0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(frameSize)))
	}

	bins := frameSize/2 + 1
	estimator := newIMCRA(bins, rate/hopSize)
	prevClean := make([]float64, bins)
	prevGain := make([]float64, bins)
	for i := range prevGain {
		prevGain[i] = 1
	}
	alphaDD := 0.98

	for f := 0; f < frameCount; f++ {
		start := f * hopSize
		spectrum := make([]complex128, frameSize)
		power := make([]float64, bins)
		for i := 0; i < frameSize; i++ {
			spectrum[i] = complex(x[start+i]*window[i], 0)
		}
		fft(spectrum, false)
		for k := 0; k < bins; k++ {
			power[k] = cmplx.Abs(spectrum[k])
			power[k] *= power[k]
		}
		noise, speechProb := estimator.Update(power)
		for k := 0; k < bins; k++ {
			gamma := clamp(power[k]/math.Max(noise[k], epsilon), 0, 100)
			xi := alphaDD*prevClean[k]/math.Max(noise[k], epsilon) + (1-alphaDD)*math.Max(gamma-1, 0)
			xi = clamp(xi, 1e-3, 100)
			v := gamma * xi / (1 + xi)
			var gain float64
			switch method {
			case MethodOMLSA:
				lsa := xi / (1 + xi) * math.Exp(0.5*expIntegralE1(math.Max(v, 1e-8)))
				// Probabilitas dari likelihood-ratio digabung dengan IMCRA.
				likelihood := math.Exp(math.Min(v, 50)) / (1 + xi)
				priorSpeech := clamp(0.15+0.8*speechProb[k], 0.05, 0.99)
				p := likelihood / (likelihood + (1-priorSpeech)/priorSpeech)
				gain = math.Pow(clamp(lsa, 0.03, 1), p) * math.Pow(0.03, 1-p)
			case MethodLogMMSE:
				gain = xi / (1 + xi) * math.Exp(0.5*expIntegralE1(math.Max(v, 1e-8)))
				gain = clamp(gain, 0.04, 1)
			default:
				gain = clamp(xi/(1+xi), 0.05, 1)
			}
			// Penghalusan waktu mengurangi musical noise.
			gain = math.Max(gain, 0.65*prevGain[k])
			prevGain[k] = gain
			prevClean[k] = gain * gain * power[k]
			spectrum[k] *= complex(gain, 0)
			if k > 0 && k < frameSize/2 {
				spectrum[frameSize-k] = cmplx.Conj(spectrum[k])
			}
		}
		fft(spectrum, true)
		for i := 0; i < frameSize; i++ {
			w := window[i]
			out[start+i] += real(spectrum[i]) * w
			norm[start+i] += w * w
		}
	}
	result := make([]float64, len(input))
	for i := range result {
		idx := i + pad
		if norm[idx] > epsilon {
			result[i] = softLimit(out[idx] / norm[idx])
		}
	}
	return result
}

type imcraEstimator struct {
	noise, smooth, minima  []float64
	minAge, framesPerReset int
	initialized            bool
}

func newIMCRA(bins, framesPerSecond int) *imcraEstimator {
	return &imcraEstimator{
		noise: make([]float64, bins), smooth: make([]float64, bins),
		minima: make([]float64, bins), framesPerReset: max(8, framesPerSecond*2),
	}
}

// Update adalah estimator minimum-controlled recursive averaging dua kondisi:
// pembaruan noise cepat saat speech absent dan lambat saat speech present.
func (e *imcraEstimator) Update(power []float64) ([]float64, []float64) {
	p := make([]float64, len(power))
	if !e.initialized {
		copy(e.noise, power)
		copy(e.smooth, power)
		copy(e.minima, power)
		for i := range e.noise {
			e.noise[i] = math.Max(e.noise[i], epsilon)
			e.minima[i] = e.noise[i]
		}
		e.initialized = true
		return append([]float64(nil), e.noise...), p
	}
	reset := e.minAge >= e.framesPerReset
	for k, value := range power {
		// Perataan frekuensi lokal dan waktu membentuk statistik IMCRA.
		local := value
		if k > 0 {
			local += 0.25 * power[k-1]
		}
		if k+1 < len(power) {
			local += 0.25 * power[k+1]
		}
		if k > 0 && k+1 < len(power) {
			local /= 1.5
		}
		e.smooth[k] = 0.8*e.smooth[k] + 0.2*local
		if reset {
			e.minima[k] = e.smooth[k]
		} else if e.smooth[k] < e.minima[k] {
			e.minima[k] = e.smooth[k]
		}
		ratio := e.smooth[k] / math.Max(e.minima[k], epsilon)
		p[k] = 1 / (1 + math.Exp(-2.5*(ratio-2.0)))
		alpha := 0.75 + 0.245*p[k]
		e.noise[k] = alpha*e.noise[k] + (1-alpha)*value
		e.noise[k] = math.Max(e.noise[k], epsilon)
	}
	if reset {
		e.minAge = 0
	} else {
		e.minAge++
	}
	return append([]float64(nil), e.noise...), p
}

func precondition(input []float64, rate float64) []float64 {
	x := append([]float64(nil), input...)
	x = applyBiquad(x, highPass(rate, 80, 0.707))
	x = applyBiquad(x, notch(rate, 50, 25))
	x = applyBiquad(x, notch(rate, 100, 25))
	return x
}

type biquad struct{ b0, b1, b2, a1, a2 float64 }

func highPass(rate, freq, q float64) biquad {
	w, alpha := 2*math.Pi*freq/rate, math.Sin(2*math.Pi*freq/rate)/(2*q)
	c := math.Cos(w)
	a0 := 1 + alpha
	return biquad{(1 + c) / 2 / a0, -(1 + c) / a0, (1 + c) / 2 / a0, -2 * c / a0, (1 - alpha) / a0}
}

func notch(rate, freq, q float64) biquad {
	w := 2 * math.Pi * freq / rate
	alpha := math.Sin(w) / (2 * q)
	c := math.Cos(w)
	a0 := 1 + alpha
	return biquad{1 / a0, -2 * c / a0, 1 / a0, -2 * c / a0, (1 - alpha) / a0}
}

func applyBiquad(x []float64, b biquad) []float64 {
	y := make([]float64, len(x))
	var x1, x2, y1, y2 float64
	for i, v := range x {
		y[i] = b.b0*v + b.b1*x1 + b.b2*x2 - b.a1*y1 - b.a2*y2
		x2 = x1
		x1 = v
		y2 = y1
		y1 = y[i]
	}
	return y
}

// expIntegralE1 menghitung E1(x), yang digunakan estimator log-spectral amplitude.
func expIntegralE1(x float64) float64 {
	if x <= 1 {
		sum, term := 0.0, 1.0
		for k := 1; k < 80; k++ {
			term *= -x / float64(k)
			add := -term / float64(k)
			sum += add
			if math.Abs(add) < 1e-14 {
				break
			}
		}
		return -0.5772156649015329 - math.Log(x) + sum
	}
	// Continued fraction (Lentz) untuk x > 1.
	b, c, d, h := x+1, 1e30, 1/(x+1), 1/(x+1)
	for i := 1; i < 100; i++ {
		a := -float64(i * i)
		b += 2
		d = 1 / (a*d + b)
		c = b + a/c
		delta := c * d
		h *= delta
		if math.Abs(delta-1) < 1e-12 {
			break
		}
	}
	return h * math.Exp(-x)
}

func softLimit(x float64) float64     { return math.Tanh(x*1.1) / math.Tanh(1.1) }
func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
