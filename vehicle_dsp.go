package main

import (
	"math"
	"math/cmplx"
)

// SuppressVehicleNoise ditujukan untuk noise kendaraan broadband yang berubah
// perlahan. calibrationSamples harus berisi noise latar tanpa ucapan.
func SuppressVehicleNoise(input []float64, rate, calibrationSamples int) []float64 {
	if len(input) == 0 {
		return nil
	}
	calibrationSamples = min(calibrationSamples, len(input))
	if calibrationSamples < frameSize {
		calibrationSamples = min(frameSize, len(input))
	}

	conditioned := vehiclePrecondition(input, float64(rate))
	window := sqrtHann(frameSize)
	bins := frameSize/2 + 1
	noise := calibratedNoiseProfile(conditioned[:calibrationSamples], window, bins)

	pad := frameSize
	paddedLen := len(conditioned) + 2*pad
	frameCount := 1 + (paddedLen-frameSize+hopSize-1)/hopSize
	total := (frameCount-1)*hopSize + frameSize
	x := make([]float64, total)
	copy(x[pad:], conditioned)
	out, norm := make([]float64, total), make([]float64, total)
	previousGain := make([]float64, bins)
	for k := range previousGain {
		previousGain[k] = 0.12
	}

	spectrum := make([]complex128, frameSize)
	power := make([]float64, bins)
	rawGain := make([]float64, bins)
	smoothGain := make([]float64, bins)
	calibrationEnd := calibrationSamples + pad

	for frame := 0; frame < frameCount; frame++ {
		start := frame * hopSize
		for i := 0; i < frameSize; i++ {
			spectrum[i] = complex(x[start+i]*window[i], 0)
		}
		fft(spectrum, false)
		for k := 0; k < bins; k++ {
			magnitude := cmplx.Abs(spectrum[k])
			power[k] = magnitude * magnitude
		}

		vad := vehicleVoiceActivity(power, noise, rate)
		frameCenter := start + frameSize/2
		withinSignal := frameCenter >= pad && frameCenter < pad+len(conditioned)
		if withinSignal && frameCenter >= calibrationEnd {
			updateVehicleNoise(noise, power, rate, vad)
		}

		for k := 0; k < bins; k++ {
			frequency := float64(k) * float64(rate) / frameSize
			alpha, floor := vehicleBandParameters(frequency)
			residualPower := math.Max(power[k]-alpha*noise[k], floor*power[k])
			gain := math.Sqrt(residualPower / math.Max(power[k], epsilon))
			if vad && frequency >= 250 && frequency <= 4000 {
				localSNRDB := 10 * math.Log10(power[k]/math.Max(noise[k], epsilon))
				confidence := clamp((localSNRDB-3)/15, 0, 1)
				gain = math.Max(gain, 0.12+0.58*confidence)
			}
			rawGain[k] = clamp(gain, 0.04, 1)
		}

		// Perataan lintas lima bin mengurangi isolated tonal holes dan musical noise.
		for k := 0; k < bins; k++ {
			sum, weight := 0.0, 0.0
			for offset := -2; offset <= 2; offset++ {
				index := k + offset
				if index < 0 || index >= bins {
					continue
				}
				w := 3.0 - math.Abs(float64(offset))
				sum += w * rawGain[index]
				weight += w
			}
			smoothGain[k] = sum / weight
		}

		for k := 0; k < bins; k++ {
			target := smoothGain[k]
			// Attack lebih cepat daripada release: noise segera ditekan, tetapi gain
			// kembali secara halus agar tidak terdengar bergetar.
			if target < previousGain[k] {
				target = 0.35*previousGain[k] + 0.65*target
			} else {
				target = 0.75*previousGain[k] + 0.25*target
			}
			previousGain[k] = target
			spectrum[k] *= complex(target, 0)
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
		index := i + pad
		if norm[index] > epsilon {
			result[i] = out[index] / norm[index]
		}
	}
	result = applyBiquad(result, lowPass(float64(rate), 8000, 0.707))
	for i := range result {
		result[i] = math.Tanh(result[i])
	}
	return result
}

func calibratedNoiseProfile(calibration, window []float64, bins int) []float64 {
	noise := make([]float64, bins)
	spectrum := make([]complex128, frameSize)
	frames := 0
	for start := 0; start+frameSize <= len(calibration); start += hopSize {
		for i := 0; i < frameSize; i++ {
			spectrum[i] = complex(calibration[start+i]*window[i], 0)
		}
		fft(spectrum, false)
		for k := 0; k < bins; k++ {
			magnitude := cmplx.Abs(spectrum[k])
			noise[k] += magnitude * magnitude
		}
		frames++
	}
	if frames == 0 {
		for k := range noise {
			noise[k] = epsilon
		}
		return noise
	}
	for k := range noise {
		noise[k] = math.Max(noise[k]/float64(frames), epsilon)
	}
	return noise
}

func vehicleVoiceActivity(power, noise []float64, rate int) bool {
	var excess, noiseEnergy float64
	for k := 0; k < len(power); k++ {
		frequency := float64(k) * float64(rate) / frameSize
		if frequency < 250 || frequency > 4000 {
			continue
		}
		excess += math.Max(power[k]-noise[k], 0)
		noiseEnergy += noise[k]
	}
	snrDB := 10 * math.Log10((excess+epsilon)/(noiseEnergy+epsilon))
	return snrDB > -3
}

func updateVehicleNoise(noise, power []float64, rate int, speechPresent bool) {
	for k := range noise {
		frequency := float64(k) * float64(rate) / frameSize
		ratio := power[k] / math.Max(noise[k], epsilon)
		alpha := 0.96
		if speechPresent && frequency >= 250 && frequency <= 4000 && ratio > 1.5 {
			alpha = 0.999
		} else if power[k] < noise[k] {
			alpha = 0.85
		}
		noise[k] = math.Max(alpha*noise[k]+(1-alpha)*power[k], epsilon)
	}
}

func vehicleBandParameters(frequency float64) (overSubtraction, spectralFloor float64) {
	switch {
	case frequency < 250:
		return 5.0, 0.015
	case frequency < 600:
		return 4.2, 0.02
	case frequency < 1500:
		return 3.4, 0.025
	case frequency < 4000:
		return 2.7, 0.03
	default:
		return 2.2, 0.04
	}
}

func vehiclePrecondition(input []float64, rate float64) []float64 {
	x := append([]float64(nil), input...)
	x = applyBiquad(x, highPass(rate, 120, 0.707))
	x = applyBiquad(x, notch(rate, 50, 25))
	x = applyBiquad(x, notch(rate, 100, 25))
	return x
}

func lowPass(rate, frequency, q float64) biquad {
	w := 2 * math.Pi * frequency / rate
	alpha := math.Sin(w) / (2 * q)
	c := math.Cos(w)
	a0 := 1 + alpha
	return biquad{(1 - c) / 2 / a0, (1 - c) / a0, (1 - c) / 2 / a0, -2 * c / a0, (1 - alpha) / a0}
}

func sqrtHann(size int) []float64 {
	window := make([]float64, size)
	for i := range window {
		window[i] = math.Sqrt(0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(size)))
	}
	return window
}
