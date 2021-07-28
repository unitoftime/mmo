package tilemap

import (
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
)

type TileType uint8

type Tile struct {
	Type TileType
	Sprite *pixel.Sprite
}

type Tilemap struct {
	TileSize int // In pixels
	tiles [][]Tile
	batch *pixel.Batch
}

func New(tiles [][]Tile, batch *pixel.Batch, tileSize int) *Tilemap {
	return &Tilemap{
		TileSize: tileSize,
		tiles: tiles,
		batch: batch,
	}
}

func (t *Tilemap) Rebatch() {
	for x := range t.tiles {
		for y := range t.tiles[x] {
			tile := t.tiles[x][y]
			pos := pixel.V(float64(x * t.TileSize), float64(y * t.TileSize))

			mat := pixel.IM.Moved(pos)
			tile.Sprite.Draw(t.batch, mat)
		}
	}
}

func (t *Tilemap) Draw(win *pixelgl.Window) {
	t.batch.Draw(win)
}
