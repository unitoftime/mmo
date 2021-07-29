package pgen

import (
	"math"

	"github.com/ojrac/opensimplex-go"
)

type Octave struct {
	Freq, Scale float64
}

type NoiseMap struct {
	seed int64
	noise opensimplex.Noise
	octaves []Octave
	exponent float64
}

func NewNoiseMap(seed int64, octaves []Octave, exponent float64) *NoiseMap {
	// TODO - ensure that sum of all octave amplitudes equals 1!
	return &NoiseMap{
		seed: seed,
		noise: opensimplex.NewNormalized(seed),
		octaves: octaves,
		exponent: exponent,
	}
}

func (n *NoiseMap) Get(x, y int) float64 {
	ret := 0.0
	for i := range n.octaves {
		xNoise := n.octaves[i].Freq * float64(x)
		yNoise := n.octaves[i].Freq * float64(y)
		ret += n.octaves[i].Scale * n.noise.Eval2(xNoise, yNoise)
	}

	ret = math.Pow(ret, n.exponent)
	return ret
}
