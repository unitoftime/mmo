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

//go:embed packed.json packed.png config.yaml man.json hat-top.json
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

	textInputMode := false

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
				// _, ok := ecs.Read[render.Sprite](world, id)
				_, ok := ecs.Read[client.Animation](world, id)
				if !ok {
					ecs.Write(world, id,
						ecs.C(client.NewAnimation(load, spritesheet, *body)),
						// ecs.C(render.NewAnimation("idle", map[string][]render.Frame{
						// 	"idle": idleAnim,
						// 	"run_left": runAnim,
						// 	"run_right": runRightAnim,
						// })),
						// ecs.C(client.HatAnimation{
						// 	Animation: render.NewAnimation("idle", map[string][]render.Frame{
						// 	"idle": hatAnim,
						// 	"run_left": hatAnim,
						// 	"run_right": hatAnim,
						// })}),
					// )
					)
					// ecs.Write(world, id,
					// 	ecs.C(render.NewSprite(
					// 		manSprites[int(body.Type)])),
					// )
				}
			})
		}},
		ecs.System{"MouseInput", func(dt time.Duration) {
			// TODO - move to other system
			_, scrollY := win.MouseScroll()
			if scrollY != 0 {
				camera.Zoom += zoomSpeed * scrollY
			}
		}},
		ecs.System{"CaptureInput", func(dt time.Duration) {
			if !textInputMode {
				// Check if they want to leave
				if win.Pressed(glitch.KeyBackspace) {
					quit.Set(true)
				}

				render.CaptureInput(win, world)
			} else {
				// Clear current inputs
				ecs.Map2(world, func(id ecs.Id, keybinds *render.Keybinds, input *physics.Input) {
					input.Left = false
					input.Right = false
					input.Up = false
					input.Down = false
				})

				if win.JustPressed(glitch.KeyEscape) {
					textInputMode = false
				}
			}
		}},
		ecs.System{"SetAnimationFromState", func(dt time.Duration) {
			ecs.Map2(world, func(id ecs.Id, input *physics.Input, anim *client.Animation) {
				if input.Left && !input.Right {
					anim.SetAnimation("run_left")
				} else if input.Right && !input.Left {
					anim.SetAnimation("run_right")
				} else if input.Up || input.Down {
					anim.SetAnimation("run_left")
				} else if input.Left && input.Right {
					anim.SetAnimation("idle_left")
				} else {
					anim.SetAnimation("idle_left")
				}
			})
		}},
	}

	physicsSystems := client.CreateClientSystems(world, sock, playerData)

	panelSprite, err := spritesheet.GetNinePanel("panel.png", glitch.R(2, 2, 2, 2))
	if err != nil { panic(err) }
	panelSprite.Scale = 8
	textInputString := ""

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

			client.PlayAnimations(pass, world, dt)
			// render.PlayAnimations(pass, world, dt)
			// ecs.Map3(world, func(id ecs.Id, anim *client.HatAnimation, bodyAnim *render.Animation, t *physics.Transform) {
			// 	anim.Update(dt)

			// 	hatPoint := *t
			// 	// hatPoint.X += float64(anim.Offset[0])
			// 	// hatPoint.Y += float64(anim.Offset[1])
			// 	frame := bodyAnim.GetFrame()
			// 	mountPoint, ok := frame.Mount["hat"]
			// 	if ok {
			// 		hatPoint.X += float64(mountPoint[0])
			// 		hatPoint.Y += float64(mountPoint[1])
			// 	}
			// 	anim.Draw(pass, &hatPoint)
			// })

			// Draw speech bubbles
			{
				// TODO - move to physics system
				{
					commandList := make([]func(), 0)
					ecs.Map(world, func(id ecs.Id, speech *game.Speech) {
						if speech.HandleRender() {
							commandList = append(commandList,
								func() {
									// TODO - combine SpeechRender component with otherone in game.SetSpeech()
									ecs.Write(world, id, ecs.C(client.SpeechRender{
										Text: atlas.Text(speech.Text),
										RemainingDuration: 5 * time.Second,
									}))
								})
						}
					})

					for _, c := range commandList {
						c()
					}
				}

				ecs.Map2(world, func(id ecs.Id, speech *client.SpeechRender, t *physics.Transform) {

					if speech.RemainingDuration < 0 { return } // Skip the display duration has ended
					speech.RemainingDuration -= dt

					scale := float32(0.4)
					mat := glitch.Mat4Ident
					mat.Scale(scale, scale, 1.0).Translate(float32(t.X), float32(t.Y + t.Height), 0)
					bounds := speech.Text.Bounds()
					mat.Translate(scale * (-bounds.W()/2), 15, 0) // TODO - 15 should come from the body height of the character (probably divided by 2)

					col := glitch.RGBA{1, 1, 1, 1}
					pass.SetLayer(100) // TODO setup layers for in world UI
					speech.Text.DrawColorMask(pass, mat, col)
				})
			}

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

				if !textInputMode && win.JustPressed(glitch.KeyEnter) {
					textInputMode = true
				} else if textInputMode {
					inputRect := win.Bounds()
					inputRect = inputRect.CutBottom(200)
					inputRect = inputRect.CutTop(100)
					inputRect = inputRect.SliceVertical(win.Bounds().W() / 3)
					// inputRect = inputRect.SliceVertical(win.Bounds().W() / 3)
					// inputRect = inputRect.SliceHorizontal(100)
					group.SetColor(glitch.RGBA{1, 1, 1, 1})
					group.TextInput(panelSprite, &textInputString, inputRect, glitch.Vec2{0.5, 0.5}, textScale)
					if win.JustPressed(glitch.KeyEnter) {
						// Write the player's speech bubble
						// msg := playerData.SendMessage(textInputString)
						client.SetSpeech(world, atlas, playerData.Id(), textInputString)
						textInputString = textInputString[:0]
						textInputMode = false
					}
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
