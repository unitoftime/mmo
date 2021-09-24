package pgen

import (
	"sort"
	"math/rand"
	"github.com/ungerik/go3d/float64/vec2"
)

func Path(start, end vec2.T, n int, variation float64) []vec2.T {
	path := make([]vec2.T, n)

	path[0] = start
	path[len(path)-1] = end

	nVec := vec2.Sub(&end, &start)

	latVec := nVec.Normalize().Rotate90DegLeft()

	for i := 1; i < n-2; i++ {
		interpVec := vec2.Interpolate(&start, &end, rand.Float64())

		rnd := 2 * (rand.Float64() - 0.5) * variation
		lateral := latVec.Scaled(rnd)

		path[i] = vec2.Add(&interpVec, &lateral)
	}

	sort.Slice(path, func(i, j int) bool {
		ii := vec2.Sub(&start, &path[i])
		jj := vec2.Sub(&start, &path[j])
		return ii.LengthSqr() < jj.LengthSqr()
	})

	return path
}
