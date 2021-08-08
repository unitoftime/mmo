package mmo

import (
	"math"

	"github.com/jstewart7/mmo/engine/tilemap"
	"github.com/jstewart7/mmo/engine/pgen"
)

const (
	GrassTile tilemap.TileType = iota
	DirtTile
	WaterTile
)

func CreateTilemap(seed int64, mapSize, tileSize int) *tilemap.Tilemap {
	octaves := []pgen.Octave{
		pgen.Octave{0.01, 0.6},
		pgen.Octave{0.05, 0.3},
		pgen.Octave{0.1, 0.07},
		pgen.Octave{0.2, 0.02},
		pgen.Octave{0.4, 0.01},
	}
	exponent := 0.8
	terrain := pgen.NewNoiseMap(seed, octaves, exponent)

	waterLevel := 0.5
	beachLevel := waterLevel + 0.1

	islandExponent := 2.0
	tiles := make([][]tilemap.Tile, mapSize, mapSize)
	for x := range tiles {
		tiles[x] = make([]tilemap.Tile, mapSize, mapSize)
		for y := range tiles[x] {

			height := terrain.Get(x, y)

			// Modify height to represent an island
			{
				dx := float64(x)/float64(mapSize) - 0.5
				dy := float64(y)/float64(mapSize) - 0.5
				d := math.Sqrt(dx * dx + dy * dy) * 2
				d = math.Pow(d, islandExponent)
				height = (1 - d + height) / 2
			}

			if height < waterLevel {
				tiles[x][y] = tilemap.Tile{WaterTile}
			} else if height < beachLevel {
				tiles[x][y] = tilemap.Tile{DirtTile}
			} else {
				tiles[x][y] = tilemap.Tile{GrassTile}
			}
		}
	}
	tmap := tilemap.New(tiles, tileSize)

	return tmap
}
