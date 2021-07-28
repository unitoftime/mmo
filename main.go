package main

//go:generate packer --input images --stats

import (
	"os"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"

	"github.com/jstewart7/mmo/engine/asset"
	"github.com/jstewart7/mmo/engine/render"
	"github.com/jstewart7/mmo/engine/tilemap"
)

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
	if err != nil {
		panic(err)
	}

	win.SetSmooth(false)

	load := asset.NewLoad(os.DirFS("./"))

	spritesheet, err := load.Spritesheet("packed.json")
	if err != nil {
		panic(err)
	}

	manSprite, err := spritesheet.Get("man1.png")
	if err != nil {
		panic(err)
	}
	manPosition := win.Bounds().Center()

	hatManSprite, err := spritesheet.Get("man2.png")
	if err != nil {
		panic(err)
	}
	hatManPosition := win.Bounds().Center()

	people := make([]Person, 0)
	people = append(people, NewPerson(manSprite, manPosition, Keybinds{
		Up: pixelgl.KeyUp,
		Down: pixelgl.KeyDown,
		Left: pixelgl.KeyLeft,
		Right: pixelgl.KeyRight,
	}))

	people = append(people, NewPerson(hatManSprite, hatManPosition, Keybinds{
		Up: pixelgl.KeyW,
		Down: pixelgl.KeyS,
		Left: pixelgl.KeyA,
		Right: pixelgl.KeyD,
	}))


	grassSprite, err := spritesheet.Get("grass.png")
	if err != nil {
		panic(err)
	}
	tileSize := 16
	mapSize := 100
	tiles := make([][]tilemap.Tile, mapSize, mapSize)
	for x := range tiles {
		tiles[x] = make([]tilemap.Tile, mapSize, mapSize)
		for y := range tiles[x] {
			tiles[x][y] = tilemap.Tile{
				Type: 0,
				Sprite: grassSprite,
			}
		}
	}
	batch := pixel.NewBatch(&pixel.TrianglesData{}, spritesheet.Picture())
	tmap := tilemap.New(tiles, batch, tileSize)
	tmap.Rebatch()

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
