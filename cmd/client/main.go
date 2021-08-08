package main

//go:generate packer --input images --stats

import (
	"os"
	"time"
	"log"
	"context"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"

	"github.com/jstewart7/mmo"
	"github.com/jstewart7/mmo/engine/asset"
	"github.com/jstewart7/mmo/engine/render"
	"github.com/jstewart7/mmo/engine/tilemap"

	"nhooyr.io/websocket"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	// Setup Network
	url := "ws://localhost:8000"

	ctx := context.Background()
	c, resp, err := websocket.Dial(ctx, url, nil)
	check(err)

	log.Println("Connection Response:", resp)

	conn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	go func() {
		counter := byte(0)
		for {
			time.Sleep(1 * time.Second)
			n, err := conn.Write([]byte{counter})
			if err != nil {
				log.Println("Error Sending:", err)
				return
			}

			log.Println("Sent n Bytes:", n)
			counter++
		}
	}()

	// Start Pixel
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
	mapSize := 1000
	tileSize := 16
	tmap := mmo.CreateTilemap(seed, mapSize, tileSize)

	grassTile, err := spritesheet.Get("grass.png")
	check(err)
	dirtTile, err := spritesheet.Get("dirt.png")
	check(err)
	waterTile, err := spritesheet.Get("water.png")
	check(err)

	tmapRender := render.NewTilemapRender(spritesheet, map[tilemap.TileType]*pixel.Sprite{
		mmo.GrassTile: grassTile,
		mmo.DirtTile: dirtTile,
		mmo.WaterTile: waterTile,
	})

	tmapRender.Batch(tmap)

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
		tmapRender.Draw(win)
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
