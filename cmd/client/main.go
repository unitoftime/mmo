package main

//go:generate packer --input images --stats

import (
	"os"
	"time"
	"log"
	"context"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"

	"nhooyr.io/websocket"

	"github.com/jstewart7/ecs"

	"github.com/jstewart7/mmo"
	"github.com/jstewart7/mmo/serdes"
	"github.com/jstewart7/mmo/engine/asset"
	"github.com/jstewart7/mmo/engine/render"
	"github.com/jstewart7/mmo/engine/physics"
	"github.com/jstewart7/mmo/engine/tilemap"
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

	networkChannel := make(chan serdes.WorldUpdate, 1024)

	world := ecs.NewWorld()
	go mmo.ClientReceive(world, conn, networkChannel)

	tmap := mmo.LoadGame(world)

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
		mmo.CreatePollNetworkSystem(world, networkChannel),
		ecs.System{"InterpolateSpritePositions", func(dt time.Duration) {
			render.InterpolateSpritePositions(world, dt)
		}},
		ecs.System{"BodyToSprite", func(dt time.Duration) {
			// TODO - would like to create the entire entity at once
			view := ecs.ViewAll(world, &mmo.Body{})
			view.Map(func(id ecs.Id, comp ...interface{}) {
				// TODO - We should really have a login-response-handling function
				sprite := render.Sprite{}
				ok := ecs.Read(world, id, &sprite)
				if !ok {
					ecs.Write(world, id, render.Sprite{
						Position: pixel.ZV, // TODO - just read this from transform
						Sprite: manSprite,
					})

					// TODO - put into a login message
					ecs.Write(world, id, physics.Input{})
					ecs.Write(world, id, render.Keybinds{
						Up: pixelgl.KeyW,
						Down: pixelgl.KeyS,
						Left: pixelgl.KeyA,
						Right: pixelgl.KeyD,
					})
				}
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
			render.CaptureInput(win, world)
		}},
	}

	physicsSystems := mmo.CreateClientSystems(world, conn)

	renderSystems := []ecs.System{
		ecs.System{"UpdateCamera", func(dt time.Duration) {
			view := ecs.ViewAll(world, &mmo.ClientOwned{}, &render.Sprite{})
			view.Map(func(id ecs.Id, comp ...interface{}) {
				sprite := comp[1].(*render.Sprite)
				camera.Position = sprite.Position
			})
			// 	// ecs.Each(engine, mmo.ClientOwned{}, func(id ecs.Id, a interface{}) {
			// 	sprite := render.Sprite{}
			// 	ok := ecs.Read(engine, id, &sprite)
			// 	if ok {
			// 		camera.Position = sprite.Position
			// 	}
			// })

			camera.Update()
		}},
		ecs.System{"Draw", func(dt time.Duration) {
			win.Clear(pixel.RGB(0, 0, 0))

			win.SetMatrix(camera.Mat())
			tmapRender.Draw(win)

			render.DrawSprites(win, world)

			win.SetMatrix(pixel.IM)
		}},
		ecs.System{"UpdateWindow", func(dt time.Duration) {
			win.Update()
		}},
	}

	ecs.RunGame(inputSystems, physicsSystems, renderSystems, &quit)
}
