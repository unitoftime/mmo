package main

//go:generate packer --input images --stats

import (
	"os"
	"time"
	"fmt"
	"embed"

	"runtime"
	"runtime/pprof"
	"flag"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"

	"github.com/unitoftime/glitch"
	"github.com/unitoftime/glitch/shaders"
	"github.com/unitoftime/glitch/ui"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/client"
	"github.com/unitoftime/mmo/mnet"
	"github.com/unitoftime/mmo/game"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/flow/asset"
	"github.com/unitoftime/flow/render"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/flow/tile"
)

//go:embed packed.json packed.png config.yaml
var fs embed.FS

type Config struct {
	Uri string
	Test bool
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
var skipMenu = flag.Bool("skip", false, "skip the login menu (for testing)")

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(fmt.Sprintf("could not create CPU profile: ", err))
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			panic(fmt.Sprintf("could not start CPU profile: ", err))
		}
		defer pprof.StopCPUProfile()
	}

	// log.SetFlags(log.LstdFlags | log.Lshortfile)

	glitch.Run(launch)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			panic(fmt.Sprintf("could not create memory profile: ", err))
		}
		defer f.Close() // error handling omitted for example
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			panic(fmt.Sprintf("could not write memory profile: ", err))
		}
	}
}

func launch() {
	win, err := glitch.NewWindow(1920, 1080, "MMO", glitch.WindowConfig{
		Vsync: true,
	})

	check(err)
	// win.SetSmooth(false)

	// camera := glitch.NewCameraOrtho()
	// camera.SetOrtho2D(win)
	// camera.SetView2D(0, 0, 1.0, 1.0)

	load := asset.NewLoad(fs)
	// load := asset.NewLoad(os.DirFS("http://localhost:8081"))
	spritesheet, err := load.Spritesheet("packed.json", false)
	check(err)

	atlas, err := glitch.DefaultAtlas()
	if err != nil { panic(err) }

	shader, err := glitch.NewShader(shaders.SpriteShader)
	if err != nil { panic(err) }

	if skipMenu == nil || (*skipMenu == false) {
		runMenu(win, load, spritesheet, shader, atlas)
	} else {
		runGame(win, load, spritesheet, shader, atlas)
	}
}

func runMenu(win *glitch.Window, load *asset.Load, spritesheet *asset.Spritesheet, shader *glitch.Shader, atlas *glitch.Atlas) {
	panelSprite, err := spritesheet.GetNinePanel("panel.png", glitch.R(2, 2, 2, 2))
	if err != nil { panic(err) }
	buttonSprite, err := spritesheet.GetNinePanel("button.png", glitch.R(1, 1, 1, 1))
	if err != nil { panic(err) }
	buttonHoverSprite, err := spritesheet.GetNinePanel("button_hover.png", glitch.R(1, 1, 1, 1))
	if err != nil { panic(err) }
	buttonPressSprite, err := spritesheet.GetNinePanel("button_press.png", glitch.R(1, 1, 1, 1))
	if err != nil { panic(err) }

	panelSprite.Scale = 8
	buttonSprite.Scale = 8
	buttonHoverSprite.Scale = 8
	buttonPressSprite.Scale = 8

	camera := glitch.NewCameraOrtho()
	camera.SetOrtho2D(win.Bounds())
	camera.SetView2D(0, 0, 1.0, 1.0)
	group := ui.NewGroup(win, camera, atlas)

	quit := ecs.Signal{}
	quit.Set(false)
	renderSystems := []ecs.System{
		ecs.System{"UpdateWindow", func(dt time.Duration) {
			if win.JustPressed(glitch.KeyBackspace) {
				quit.Set(true)
			}

			glitch.Clear(win, glitch.Black)

			{
				ui.Clear()
				group.Clear()
				camera.SetOrtho2D(win.Bounds())
				camera.SetView2D(0, 0, 1.0, 1.0)

				menuRect := win.Bounds().SliceHorizontal(500).SliceVertical(500)
				group.Panel(panelSprite, menuRect)

				paddingRect := glitch.R(-10,-10,-10,-10)
				buttonHeight := float32(50)
				buttonWidth := float32(200)

				// Play button
				{
					buttonRect := menuRect.SliceHorizontal(buttonHeight).SliceVertical(buttonWidth).Moved(glitch.Vec2{0, buttonHeight})
					if group.Button(buttonSprite, buttonHoverSprite, buttonPressSprite, buttonRect) {
						runGame(win, load, spritesheet, shader, atlas)
					}
					group.SetColor(glitch.RGBA{0, 0, 0, 1})
					group.Text("Play", buttonRect.Pad(paddingRect), glitch.Vec2{0.5, 0.5})
				}

				// Exit button
				{
					buttonRect := menuRect.SliceHorizontal(buttonHeight).SliceVertical(buttonWidth).Moved(glitch.Vec2{0, -buttonHeight})
					if group.Button(buttonSprite, buttonHoverSprite, buttonPressSprite, buttonRect) {
						quit.Set(true)
					}
					group.SetColor(glitch.RGBA{0, 0, 0, 1})
					group.Text("Exit", buttonRect.Pad(paddingRect), glitch.Vec2{0.5, 0.5})
				}

				group.Draw()
			}

			win.Update()
		}},
	}
	ecs.RunGame(nil, nil, renderSystems, &quit)
}

func runGame(win *glitch.Window, load *asset.Load, spritesheet *asset.Spritesheet, shader *glitch.Shader, atlas *glitch.Atlas) {
	world := ecs.NewWorld()
	networkChannel := make(chan serdes.WorldUpdate, 1024)

	config := Config{}
	err := load.Yaml("config.yaml", &config)
	if err != nil { panic(err) }
	log.Print(config)

	url := config.Uri
	sock, err := mnet.NewSocket(url, config.Test)
	if err != nil {
		panic(err)
	}
	// sock.SkipVerify = config.Test

	// This is the player's ID, by default we set this to invalid
	playerData := mmo.NewPlayerData()

	go mnet.ReconnectLoop(sock, func(sock *mnet.Socket) error {
		return client.ClientReceive(sock, playerData, networkChannel)
	})

	pass := glitch.NewRenderPass(shader)
	tilemapPass := glitch.NewRenderPass(shader)

	tmap := mmo.LoadGame(world)

	grassTile, err := spritesheet.Get("grass.png")
	check(err)
	dirtTile, err := spritesheet.Get("dirt.png")
	check(err)
	waterTile, err := spritesheet.Get("water.png")
	check(err)
	log.Print(*waterTile)

	tmapRender := render.NewTilemapRender(spritesheet, map[tile.TileType]*glitch.Sprite{
		mmo.GrassTile: grassTile,
		mmo.DirtTile: dirtTile,
		mmo.WaterTile: waterTile,
	}, tilemapPass)

	tmapRender.Clear()
	tmapRender.Batch(tmap)

	// Create people
	manSprites := make([]*glitch.Sprite, game.NumBodyTypes)
	for i := 0; i < len(manSprites); i++ {
		manSprites[i], err = spritesheet.Get(fmt.Sprintf("man%d.png", i))
		check(err)
	}

	screenCamera := glitch.NewCameraOrtho()
	screenCamera.SetOrtho2D(win.Bounds())
	screenCamera.SetView2D(0, 0, 1.0, 1.0)
	group := ui.NewGroup(win, screenCamera, atlas)

	camera := render.NewCamera(win.Bounds(), 0, 0)
	zoomSpeed := 0.1
	quit := ecs.Signal{}
	quit.Set(false)

	inputSystems := []ecs.System{
		mmo.CreatePollNetworkSystem(world, networkChannel),
		// ecs.System{"InterpolateSpritePositions", func(dt time.Duration) {
		// 	render.InterpolateSpritePositions(world, dt)
		// }},
		ecs.System{"ManageEntityTimeout", func(dt time.Duration) {
			timeout := 5 * time.Second
			now := time.Now()
			ecs.Map(world, func(id ecs.Id, lastUpdate *mmo.LastUpdate) {
				if now.Sub(lastUpdate.Time) > timeout {
					ecs.Delete(world, id)
				}
			})
		}},
		ecs.System{"BodyToSprite", func(dt time.Duration) {
			ecs.Map(world, func(id ecs.Id, body *game.Body) {
				_, ok := ecs.Read[render.Sprite](world, id)
				if !ok {
					ecs.Write(world, id,
						ecs.C(render.NewSprite(
							manSprites[int(body.Type)])),
					)
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

	physicsSystems := client.CreateClientSystems(world, sock, playerData)

	renderSystems := []ecs.System{
		ecs.System{"UpdateCamera", func(dt time.Duration) {
			transform, ok := ecs.Read[physics.Transform](world, playerData.Id())
			if ok {
				// log.Print("Update Camera", transform)
				// sprite := comp[1].(*render.Sprite)
				// camera.Position = sprite.Position
				camera.Position = glitch.Vec2{float32(transform.X), float32(transform.Y)}
			}

			// ecs.Map2(world, func(id ecs.Id, _ *mmo.ClientOwned, transform *physics.Transform) {
			// 	log.Println("Update Camera", transform)
			// 	// sprite := comp[1].(*render.Sprite)
			// 	// camera.Position = sprite.Position
			// 	camera.Position = glitch.Vec2{float32(transform.X), float32(transform.Y)}
			// })

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

			// Draw UI
			ui.Clear()
			{
				group.Clear()
				screenCamera.SetOrtho2D(win.Bounds())
				screenCamera.SetView2D(0, 0, 1.0, 1.0)

				paddingRect := glitch.R(-10,-10,-10,-10)
				connectedRect := win.Bounds()
				connectedRect = connectedRect.Pad(paddingRect)
				textScale := float32(0.5)
				if sock.Connected.Load() {
					group.SetColor(glitch.RGBA{0, 1, 0, 1})
					group.FixedText("Connected", connectedRect, glitch.Vec2{1, 0}, textScale)
				} else {
					group.SetColor(glitch.RGBA{1, 0, 0, 1})
					group.FixedText("Disconnected", connectedRect, glitch.Vec2{1, 0}, textScale)
				}
				group.Draw()
			}
		}},
		ecs.System{"UpdateWindow", func(dt time.Duration) {
			win.Update()
		}},
	}

	ecs.RunGame(inputSystems, physicsSystems, renderSystems, &quit)
	log.Print("Finished ecs.RunGame")

	// TODO - I'm not sure if this is the proper way to close because `ClientReceive` is still reading, so closing here will cause that to fail
	sock.Close()
}
