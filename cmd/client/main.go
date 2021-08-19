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
	"github.com/jstewart7/mmo/engine/physics"
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

	engine := ecs.NewEngine()
	tmap, manId, hatManId := mmo.LoadGame(engine)

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
	// TODO - move to system that converts like bodytype into sprites
	manSprite, err := spritesheet.Get("man1.png")
	check(err)

	hatManSprite, err := spritesheet.Get("man2.png")
	check(err)

	ecs.Write(engine, manId, render.Sprite{manSprite})
	ecs.Write(engine, manId, render.Keybinds{
		Up: pixelgl.KeyUp,
		Down: pixelgl.KeyDown,
		Left: pixelgl.KeyLeft,
		Right: pixelgl.KeyRight,
	})

	ecs.Write(engine, hatManId, render.Sprite{hatManSprite})
	ecs.Write(engine, hatManId, render.Keybinds{
		Up: pixelgl.KeyW,
		Down: pixelgl.KeyS,
		Left: pixelgl.KeyA,
		Right: pixelgl.KeyD,
	})

	camera := render.NewCamera(win, 0, 0)
	zoomSpeed := 0.1
	quit := ecs.Signal{}
	quit.Set(false)

	inputSystems := []ecs.System{
		ecs.System{"Clear", func(dt time.Duration) {
			win.Clear(pixel.RGB(0, 0, 0))

			// TODO - move to other system
			scroll := win.MouseScroll()
			if scroll.Y != 0 {
				camera.Zoom += zoomSpeed * scroll.Y
			}

			if win.JustPressed(pixelgl.KeyEscape) {
				quit.Set(true)
			}
		}},
		ecs.System{"CaptureInput", func(dt time.Duration) {
			render.CaptureInput(win, engine)
		}},
	}

	physicsSystems := mmo.CreatePhysicsSystems(engine)

	renderSystems := []ecs.System{
		ecs.System{"UpdateCamera", func(dt time.Duration) {
			transform := physics.Transform{}
			ok := ecs.Read(engine, manId, &transform)
			if ok {
				camera.Position = pixel.V(transform.X, transform.Y)
			}
			camera.Update()
		}},
		ecs.System{"Draw", func(dt time.Duration) {
			win.SetMatrix(camera.Mat())
			tmapRender.Draw(win)

			render.DrawSprites(win, engine)

			win.SetMatrix(pixel.IM)
		}},
		ecs.System{"UpdateWindow", func(dt time.Duration) {
			win.Update()
		}},
	}

	ecs.RunGame(inputSystems, physicsSystems, renderSystems, &quit)
}

