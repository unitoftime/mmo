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
	pixelgl.Run(runGame)
}

func runGame() {
	// Setup Network
	url := "ws://localhost:8000"

	ctx := context.Background()
	c, resp, err := websocket.Dial(ctx, url, nil)
	check(err)

	log.Println("Connection Response:", resp)

	conn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	// Setup Pixel
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
	go mmo.ClientReceive(engine, conn)

	tmap := mmo.LoadGame(engine)

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

	camera := render.NewCamera(win, 0, 0)
	zoomSpeed := 0.1
	quit := ecs.Signal{}
	quit.Set(false)

	inputSystems := []ecs.System{
		ecs.System{"BodyToSprite", func(dt time.Duration) {
			ecs.Each(engine, mmo.Body{}, func(id ecs.Id, a interface{}) {
				ecs.Write(engine, id, render.Sprite{manSprite})
				ecs.Write(engine, id, physics.Input{})
				ecs.Write(engine, id, render.Keybinds{
					Up: pixelgl.KeyW,
					Down: pixelgl.KeyS,
					Left: pixelgl.KeyA,
					Right: pixelgl.KeyD,
				})
			})
		}},
		ecs.System{"MouseInput", func(dt time.Duration) {
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

	physicsSystems := mmo.CreateClientSystems(engine, conn)

	renderSystems := []ecs.System{
		ecs.System{"UpdateCamera", func(dt time.Duration) {
			// transform := physics.Transform{}
			// ok := ecs.Read(engine, manId, &transform)
			// if ok {
			// 	camera.Position = pixel.V(transform.X, transform.Y)
			// }
			// camera.Update()
		}},
		ecs.System{"Draw", func(dt time.Duration) {
			win.Clear(pixel.RGB(0, 0, 0))

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

