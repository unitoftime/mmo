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
	"github.com/jstewart7/mmo/engine/ecs"
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

	engine := ecs.NewEngine()

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
	spawnPoint := Transform{
		float64(tileSize*mapSize/2),
		float64(tileSize*mapSize/2)}

	manSprite, err := spritesheet.Get("man1.png")
	check(err)

	hatManSprite, err := spritesheet.Get("man2.png")
	check(err)

	manId := engine.NewId()
	ecs.Write(engine, manId, Sprite{manSprite})
	ecs.Write(engine, manId, spawnPoint)
	ecs.Write(engine, manId, Keybinds{
		Up: pixelgl.KeyUp,
		Down: pixelgl.KeyDown,
		Left: pixelgl.KeyLeft,
		Right: pixelgl.KeyRight,
	})

	hatManId := engine.NewId()
	ecs.Write(engine, hatManId, Sprite{hatManSprite})
	ecs.Write(engine, hatManId, spawnPoint)
	ecs.Write(engine, hatManId, Keybinds{
		Up: pixelgl.KeyW,
		Down: pixelgl.KeyS,
		Left: pixelgl.KeyA,
		Right: pixelgl.KeyD,
	})


	camera := render.NewCamera(win, 0, 0)
	zoomSpeed := 0.1
	for !win.JustPressed(pixelgl.KeyEscape) {
		win.Clear(pixel.RGB(0, 0, 0))

		scroll := win.MouseScroll()
		if scroll.Y != 0 {
			camera.Zoom += zoomSpeed * scroll.Y
		}

		HandleInput(win, engine)

		transform := Transform{}
		ok := ecs.Read(engine, manId, &transform)
		if ok {
			camera.Position = pixel.V(transform.X, transform.Y)
		}
		camera.Update()

		win.SetMatrix(camera.Mat())
		tmapRender.Draw(win)

		DrawSprites(win, engine)

		win.SetMatrix(pixel.IM)

		win.Update()
	}
}

type Keybinds struct {
	Up, Down, Left, Right pixelgl.Button
}
func (t *Keybinds) ComponentSet(val interface{}) { *t = val.(Keybinds) }

type Sprite struct {
	*pixel.Sprite
}
func (t *Sprite) ComponentSet(val interface{}) { *t = val.(Sprite) }

type Transform struct {
	X, Y float64
}
func (t *Transform) ComponentSet(val interface{}) { *t = val.(Transform) }

func DrawSprites(win *pixelgl.Window, engine *ecs.Engine) {
	ecs.Each(engine, Sprite{}, func(id ecs.Id, a interface{}) {
		sprite := a.(Sprite)

		transform := Transform{}
		ok := ecs.Read(engine, id, &transform)
		if !ok { return }

		pos := pixel.V(transform.X, transform.Y)
		sprite.Draw(win, pixel.IM.Scaled(pixel.ZV, 2.0).Moved(pos))
	})
}

func HandleInput(win *pixelgl.Window, engine *ecs.Engine) {
	ecs.Each(engine, Keybinds{}, func(id ecs.Id, a interface{}) {
		keybinds := a.(Keybinds)

		transform := Transform{}
		ok := ecs.Read(engine, id, &transform)
		if !ok { return }

		if win.Pressed(keybinds.Left) {
			transform.X -= 2.0
		}
		if win.Pressed(keybinds.Right) {
			transform.X += 2.0
		}
		if win.Pressed(keybinds.Up) {
			transform.Y += 2.0
		}
		if win.Pressed(keybinds.Down) {
			transform.Y -= 2.0
		}

		ecs.Write(engine, id, transform)
	})
}
