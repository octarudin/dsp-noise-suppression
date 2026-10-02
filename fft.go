package main

import (
	"math"
	"math/cmplx"
)

func fft(x []complex128, inverse bool) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		if inverse {
			angle = -angle
		}
		step := cmplx.Rect(1, angle)
		for start := 0; start < n; start += length {
			w := complex(1.0, 0)
			half := length / 2
			for j := 0; j < half; j++ {
				u := x[start+j]
				v := x[start+j+half] * w
				x[start+j], x[start+j+half] = u+v, u-v
				w *= step
			}
		}
	}
	if inverse {
		scale := complex(float64(n), 0)
		for i := range x {
			x[i] /= scale
		}
	}
}
