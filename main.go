package main

//go:generate packer --input images --stats

import (
	"os"
	"time"
	"math"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"

	"github.com/jstewart7/mmo/engine/asset"
	"github.com/jstewart7/mmo/engine/render"
	"github.com/jstewart7/mmo/engine/tilemap"
	"github.com/jstewart7/mmo/engine/pgen"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	pixelgl.Run(runGame)
}

func runGame() {
	cfg := pixelgl.WindowConfig{
		Title: "MMO",
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync: true,
		Resizable: true,
	}

	win, err := pixelgl.NewWindow(cfg)
	check(err)

	win.SetSmooth(false)

	load := asset.NewLoad(os.DirFS("./"))

	spritesheet, err := load.Spritesheet("packed.json")
	check(err)

	// Create Tilemap
	seed := time.Now().UTC().UnixNano()
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
	tileSize := 16
	mapSize := 1000
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
				tiles[x][y] = GetTile(spritesheet, WaterTile)
			} else if height < beachLevel {
				tiles[x][y] = GetTile(spritesheet, DirtTile)
			} else {
				tiles[x][y] = GetTile(spritesheet, GrassTile)
			}
		}
	}
	batch := pixel.NewBatch(&pixel.TrianglesData{}, spritesheet.Picture())
	tmap := tilemap.New(tiles, batch, tileSize)
	tmap.Rebatch()

	// Create people
	spawnPoint := pixel.V(
		float64(tileSize*mapSize/2),
		float64(tileSize*mapSize/2))

	manSprite, err := spritesheet.Get("man1.png")
	check(err)

	hatManSprite, err := spritesheet.Get("man2.png")
	check(err)

	people := make([]Person, 0)
	people = append(people, NewPerson(manSprite, spawnPoint, Keybinds{
		Up: pixelgl.KeyUp,
		Down: pixelgl.KeyDown,
		Left: pixelgl.KeyLeft,
		Right: pixelgl.KeyRight,
	}))

	people = append(people, NewPerson(hatManSprite, spawnPoint, Keybinds{
		Up: pixelgl.KeyW,
		Down: pixelgl.KeyS,
		Left: pixelgl.KeyA,
		Right: pixelgl.KeyD,
	}))

	camera := render.NewCamera(win, 0, 0)
	zoomSpeed := 0.1
	for !win.JustPressed(pixelgl.KeyEscape) {
		win.Clear(pixel.RGB(0, 0, 0))

		scroll := win.MouseScroll()
		if scroll.Y != 0 {
			camera.Zoom += zoomSpeed * scroll.Y
		}

		for i := range people {
			people[i].HandleInput(win)
		}

		camera.Position = people[0].Position
		camera.Update()

		win.SetMatrix(camera.Mat())
		tmap.Draw(win)
		for i := range people {
			people[i].Draw(win)
		}
		win.SetMatrix(pixel.IM)

		win.Update()
	}
}

const (
	GrassTile tilemap.TileType = iota
	DirtTile
	WaterTile
)

func GetTile(ss *asset.Spritesheet, t tilemap.TileType) tilemap.Tile {

	spriteName := ""
	switch t {
	case GrassTile:
		spriteName = "grass.png"
	case DirtTile:
		spriteName = "dirt.png"
	case WaterTile:
		spriteName = "water.png"
	default:
		panic("Unknow TileType!")
	}

	sprite, err := ss.Get(spriteName)
	check(err)

	return tilemap.Tile{
		Type: t,
		Sprite: sprite,
	}
}

type Keybinds struct {
	Up, Down, Left, Right pixelgl.Button
}

type Person struct {
	Sprite *pixel.Sprite
	Position pixel.Vec
	Keybinds Keybinds
}

func NewPerson(sprite *pixel.Sprite, position pixel.Vec, keybinds Keybinds) Person {
	return Person{
		Sprite: sprite,
		Position: position,
		Keybinds: keybinds,
	}
}

func (p *Person) Draw(win *pixelgl.Window) {
	p.Sprite.Draw(win, pixel.IM.Scaled(pixel.ZV, 2.0).Moved(p.Position))
}

func (p *Person) HandleInput(win *pixelgl.Window) {
	if win.Pressed(p.Keybinds.Left) {
		p.Position.X -= 2.0
	}
	if win.Pressed(p.Keybinds.Right) {
		p.Position.X += 2.0
	}
	if win.Pressed(p.Keybinds.Up) {
		p.Position.Y += 2.0
	}
	if win.Pressed(p.Keybinds.Down) {
		p.Position.Y -= 2.0
	}
}
