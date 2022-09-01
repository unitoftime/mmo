package main

//go:generate packer --input images --stats

import (
	"os"
	"time"
	"log"
	"context"
	"embed"

	"runtime"
	"runtime/pprof"
	"flag"

	"nhooyr.io/websocket"

	"github.com/unitoftime/ecs"

	"github.com/unitoftime/glitch"
	"github.com/unitoftime/glitch/shaders"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/flow/asset"
	"github.com/unitoftime/flow/render"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/flow/tile"
)

//go:embed packed.json packed.png
var f embed.FS

func check(err error) {
	if err != nil {
		panic(err)
	}
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	glitch.Run(runGame)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}

func runGame() {
	// Setup Network
	url := "ws://localhost:8000"

	ctx := context.Background()
	c, resp, err := websocket.Dial(ctx, url, nil)
	check(err)

	log.Println("Connection Response:", resp)

	conn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	win, err := glitch.NewWindow(1920, 1080, "MMO", glitch.WindowConfig{
		Vsync: true,
	})

	check(err)
	// win.SetSmooth(false)

	// camera := glitch.NewCameraOrtho()
	// camera.SetOrtho2D(win)
	// camera.SetView2D(0, 0, 1.0, 1.0)

	shader, err := glitch.NewShader(shaders.SpriteShader)
	if err != nil { panic(err) }

	pass := glitch.NewRenderPass(shader)
	tilemapPass := glitch.NewRenderPass(shader)

	load := asset.NewLoad(f)
	// load := asset.NewLoad(os.DirFS("http://localhost:8081"))
	spritesheet, err := load.Spritesheet("packed.json", false)
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
	log.Println(*waterTile)

	tmapRender := render.NewTilemapRender(spritesheet, map[tile.TileType]*glitch.Sprite{
		mmo.GrassTile: grassTile,
		mmo.DirtTile: dirtTile,
		mmo.WaterTile: waterTile,
	}, tilemapPass)

	tmapRender.Clear()
	tmapRender.Batch(tmap)

	// Create people
	// TODO - move to system that converts like bodytype into sprites
	manSprite, err := spritesheet.Get("man1.png")
	check(err)

	camera := render.NewCamera(win.Bounds(), 0, 0)
	zoomSpeed := 0.1
	quit := ecs.Signal{}
	quit.Set(false)

	inputSystems := []ecs.System{
		mmo.CreatePollNetworkSystem(world, networkChannel),
		// ecs.System{"InterpolateSpritePositions", func(dt time.Duration) {
		// 	render.InterpolateSpritePositions(world, dt)
		// }},
		ecs.System{"BodyToSprite", func(dt time.Duration) {
			// TODO - would like to create the entire entity at once
			ecs.Map(world, func(id ecs.Id, body *mmo.Body) {

				// TODO - We should really have a login-response-handling function
				// sprite := render.Sprite{}
				// ok := ecs.Read(world, id, &sprite)
				_, ok := ecs.Read[render.Sprite](world, id)
				if !ok {
					ecs.Write(world, id, ecs.C(render.NewSprite(
						// Position: pixel.ZV, // TODO - just read this from transform
						manSprite)))

					// TODO - put into a login message
					ecs.Write(world, id, ecs.C(physics.Input{}))
					ecs.Write(world, id, ecs.C(render.Keybinds{
						Up: glitch.KeyW,
						Down: glitch.KeyS,
						Left: glitch.KeyA,
						Right: glitch.KeyD,
					}))
				}
			})
		}},
		ecs.System{"MouseInput", func(dt time.Duration) {
			// TODO - move to other system
			_, scrollY := win.MouseScroll()
			if scrollY != 0 {
				camera.Zoom += zoomSpeed * scrollY
			}

			if win.Pressed(glitch.KeyBackspace) {
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
			ecs.Map2(world, func(id ecs.Id, _ *mmo.ClientOwned, transform *physics.Transform) {
				log.Println("Update Camera", transform)
				// sprite := comp[1].(*render.Sprite)
				// camera.Position = sprite.Position
				camera.Position = glitch.Vec2{float32(transform.X), float32(transform.Y)}
			})

			camera.Update(win.Bounds())
		}},
		ecs.System{"Draw", func(dt time.Duration) {
			glitch.Clear(win, glitch.RGBA{0, 0, 0, 1.0})

			// win.SetMatrix(camera.Mat())
			// tmapRender.Draw(win)

			pass.Clear()
			render.DrawSprites(pass, world)

			// win.SetMatrix(pixel.IM)
			tilemapPass.SetUniform("projection", camera.Camera.Projection)
			tilemapPass.SetUniform("view", camera.Camera.View)
			tilemapPass.Draw(win)

			pass.SetUniform("projection", camera.Camera.Projection)
			pass.SetUniform("view", camera.Camera.View)
			pass.Draw(win)
		}},
		ecs.System{"UpdateWindow", func(dt time.Duration) {
			win.Update()
		}},
	}

	ecs.RunGame(inputSystems, physicsSystems, renderSystems, &quit)
}
