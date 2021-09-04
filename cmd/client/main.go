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

	"github.com/jstewart7/mmo"
	"github.com/jstewart7/mmo/serdes"
	"github.com/jstewart7/mmo/engine/ecs"
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

	engine := ecs.NewEngine()
	go mmo.ClientReceive(engine, conn, networkChannel)

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
		mmo.CreatePollNetworkSystem(engine, networkChannel),
		ecs.System{"InterpolateSpritePositions", func(dt time.Duration) {
			render.InterpolateSpritePositions(engine, dt)
		}},
		ecs.System{"BodyToSprite", func(dt time.Duration) {
			ecs.Each(engine, mmo.Body{}, func(id ecs.Id, a interface{}) {

				// TODO - We should really have a login-response-handling function
				sprite := render.Sprite{}
				ok := ecs.Read(engine, id, &sprite)
				if !ok {
					ecs.Write(engine, id, render.Sprite{
						Position: pixel.ZV, // TODO - just read this from transform
						Sprite: manSprite,
					})

					// TODO - put into a login message
					ecs.Write(engine, id, physics.Input{})
					ecs.Write(engine, id, render.Keybinds{
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
			render.CaptureInput(win, engine)
		}},
	}

	physicsSystems := mmo.CreateClientSystems(engine, conn)

	renderSystems := []ecs.System{
		ecs.System{"UpdateCamera", func(dt time.Duration) {
			ecs.Each(engine, mmo.ClientOwned{}, func(id ecs.Id, a interface{}) {
				sprite := render.Sprite{}
				ok := ecs.Read(engine, id, &sprite)
				if ok {
					camera.Position = sprite.Position
				}
			})

			camera.Update()
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

