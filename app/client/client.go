package client

//go:generate sh ase/export.sh

import (
	"os"
	"time"
	// "fmt"
	"embed"
	// "math"

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

//go:embed assets/*
var fs embed.FS

type Config struct {
	ProxyUri string
}

var skipMenu = flag.Bool("skip", false, "skip the login menu (for testing)")

var globalConfig Config
func Main(config Config) {
	globalConfig = config

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	flag.Parse()

	glitch.Run(launch)
}

func launch() {
	win, err := glitch.NewWindow(1920, 1080, "MMO", glitch.WindowConfig{
		Vsync: true,
		// Fullscreen: true,
		// Samples: 4,
	})
	if err != nil {
		panic(err)
	}

	load := asset.NewLoad(fs)
	// load := asset.NewLoad(os.DirFS("http://localhost:8081"))
	spritesheet, err := load.Spritesheet("assets/spritesheet.json", false) // TODO - does this need to be false or true?
	if err != nil {
		panic(err)
	}

	atlas, err := glitch.DefaultAtlas()
	if err != nil { panic(err) }

	// Note: If you want fractional zooming, you can use pixelartshader, else if you do x2 zooming, you can use spriteshader
	// shader, err := glitch.NewShader(shaders.PixelArtShader)
	shader, err := glitch.NewShader(shaders.SpriteShader)
	if err != nil { panic(err) }

	if skipMenu == nil || (*skipMenu == false) {
		runMenu(win, load, spritesheet, shader, atlas)
	} else {
		runGame(win, load, spritesheet, shader, atlas)
	}
}

func runMenu(win *glitch.Window, load *asset.Load, spritesheet *asset.Spritesheet, shader *glitch.Shader, atlas *glitch.Atlas) {
	panelSprite, err := spritesheet.GetNinePanel("ui_panel0.png", glitch.R(2, 2, 2, 2))
	if err != nil { panic(err) }
	buttonSprite, err := spritesheet.GetNinePanel("ui_button0.png", glitch.R(1, 1, 1, 1))
	if err != nil { panic(err) }
	buttonHoverSprite, err := spritesheet.GetNinePanel("ui_button_hover0.png", glitch.R(1, 1, 1, 1))
	if err != nil { panic(err) }
	buttonPressSprite, err := spritesheet.GetNinePanel("ui_button_press0.png", glitch.R(1, 1, 1, 1))
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

				buttonHeight := float32(50)
				buttonWidth := float32(200)

				// Play button
				{
					buttonRect := menuRect.SliceHorizontal(buttonHeight).SliceVertical(buttonWidth).Moved(glitch.Vec2{0, buttonHeight})
					if group.Button(buttonSprite, buttonHoverSprite, buttonPressSprite, buttonRect) {
						runGame(win, load, spritesheet, shader, atlas)
					}
					group.SetColor(glitch.RGBA{0, 0, 0, 1})
					group.Text("Play", buttonRect.Unpad(buttonSprite.Border()), glitch.Vec2{0.5, 0.5})
				}

				// Exit button
				{
					buttonRect := menuRect.SliceHorizontal(buttonHeight).SliceVertical(buttonWidth).Moved(glitch.Vec2{0, -buttonHeight})
					if group.Button(buttonSprite, buttonHoverSprite, buttonPressSprite, buttonRect) {
						quit.Set(true)
					}
					group.SetColor(glitch.RGBA{0, 0, 0, 1})
					group.Text("Exit", buttonRect.Unpad(buttonSprite.Border()), glitch.Vec2{0.5, 0.5})
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

	url := globalConfig.ProxyUri
	sock, err := mnet.NewSocket(url)
	if err != nil {
		panic(err)
	}
	// sock.SkipVerify = config.Test

	// This is the player's ID, by default we set this to invalid
	playerData := mmo.NewPlayerData()

	go mnet.ReconnectLoop(sock, func(sock *mnet.Socket) error {
		return client.ClientReceive(sock, playerData, networkChannel)
	})

	// Note: This requires a system to update the framebuffer if the window is resized. The system should essentially recreate the framebuffer with the new dimensions, This might be a good target for the framebuffer callback, but for now I'm just going to poll win.Bounds
	renderBounds := win.Bounds()
	frame := glitch.NewFrame(renderBounds, false)

	windowPass := glitch.NewRenderPass(shader)

	pass := glitch.NewRenderPass(shader)
	pass.SoftwareSort = glitch.SoftwareSortY
	tilemapPass := glitch.NewRenderPass(shader)

	tilemap := mmo.LoadGame(world)

	grassTile, err := spritesheet.Get("grass0.png")
	if err != nil { panic(err) }
	dirtTile, err := spritesheet.Get("dirt0.png")
	if err != nil { panic(err) }
	waterTile, err := spritesheet.Get("water0.png")
	if err != nil { panic(err) }
	concreteTile, err := spritesheet.Get("concrete0.png")
	if err != nil { panic(err) }
	wallSprite, err := spritesheet.Get("wall0.png")
	if err != nil { panic(err) }

	tmapRender := render.NewTilemapRender(spritesheet, map[tile.TileType]*glitch.Sprite{
		mmo.GrassTile: grassTile,
		mmo.DirtTile: dirtTile,
		mmo.WaterTile: waterTile,
		mmo.ConcreteTile: concreteTile,
	}, tilemapPass)

	tmapRender.Clear()
	tmapRender.Batch(tilemap)

	textInputMode := false

	screenCamera := glitch.NewCameraOrtho()
	screenCamera.SetOrtho2D(win.Bounds())
	screenCamera.SetView2D(0, 0, 1.0, 1.0)
	group := ui.NewGroup(win, screenCamera, atlas)

	camera := render.NewCamera(win.Bounds(), 0, 0)
	// zoomSpeed := 1.0
	quit := ecs.Signal{}
	quit.Set(false)

	inputSystems := []ecs.System{
		mmo.CreatePollNetworkSystem(world, networkChannel),
		ecs.System{"ManageEntityTimeout", func(dt time.Duration) {
			timeout := 5 * time.Second
			now := time.Now()
			ecs.Map(world, func(id ecs.Id, lastUpdate *mmo.LastUpdate) {
				if now.Sub(lastUpdate.Time) > timeout {
					ecs.Delete(world, id)
				}
			})
		}},
		ecs.System{"BodySetup", func(dt time.Duration) {
			ecs.Map(world, func(id ecs.Id, body *game.Body) {
				// TODO - is there a way to not have to poll these each frame?
				// Body to animation
				_, ok := ecs.Read[client.Animation](world, id)
				if !ok {
					ecs.Write(world, id,
						ecs.C(client.NewAnimation(load, spritesheet, *body)),
					)
				}

				// Body to collider
				_, ok = ecs.Read[physics.CircleCollider](world, id)
				if !ok {
					// TODO - hardcoded here and in network.go - Centralize character creation
					// TODO - arbitrary collider radius 6
					collider := physics.NewCircleCollider(6)
					collider.Layer = mmo.BodyLayer
					collider.HitLayer = mmo.BodyLayer
					ecs.Write(world, id,
						ecs.C(collider),
						ecs.C(physics.NewColliderCache()),
					)
				}
			})

			// Tile objects?
			ecs.Map(world, func(id ecs.Id, body *game.TileObject) {
				_, ok := ecs.Read[render.Sprite](world, id)
				if !ok {
					ecs.Write(world, id,
						ecs.C(render.NewSprite(wallSprite)),
					)
				}
			})
		}},
		ecs.System{"MouseInput", func(dt time.Duration) {
			// TODO - move to other system
			_, scrollY := win.MouseScroll()

			const minZoom = 1
			const maxZoom = 16.0

			if scrollY > 0 {
				if camera.Zoom < maxZoom {
					camera.Zoom = camera.Zoom * 2
				}
			} else if scrollY < 0 {
				if camera.Zoom > minZoom {
					camera.Zoom = camera.Zoom / 2
				}
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

	physicsSystems := client.CreateClientSystems(world, sock, playerData, tilemap)

	panelSprite, err := spritesheet.GetNinePanel("ui_panel0.png", glitch.R(2, 2, 2, 2))
	if err != nil { panic(err) }
	panelSprite.Scale = 8
	textInputString := ""

	// debugSprite, err := spritesheet.Get("ui_panel0.png")
	// if err != nil { panic(err) }

	renderSystems := []ecs.System{
		ecs.System{"UpdateFramebuffer", func(dt time.Duration) {
			renderBounds := win.Bounds()
			// TODO - how to determine 50?
			renderBounds = renderBounds.Pad(glitch.R(50,50,50,50)) // Pad out by 50 pixel so that camera can drift inside pixels
			if frame.Bounds() != renderBounds {
				frame = glitch.NewFrame(renderBounds, false)
				log.Print("recreating fbo: ", frame.Bounds(), renderBounds, win.Bounds())
			}
		}},
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
			// camera.Update(renderBounds)
		}},
		ecs.System{"Draw", func(dt time.Duration) {
			glitch.Clear(win, glitch.RGBA{0, 0, 0, 1.0})

			// win.SetMatrix(camera.Mat())
			// tmapRender.Draw(win)

			pass.Clear()
			render.DrawSprites(pass, world)

			client.PlayAnimations(pass, world, dt)

			// Debug. Draw neworking position buffer
			// {
			// 	ecs.Map2(world, func(id ecs.Id, t *physics.Transform, nt *client.NextTransform) {

			// 		npos := nt.PhyTrans
			// 		mat := glitch.Mat4Ident
			// 		mat.Scale(0.5, 0.5, 1.0).Translate(float32(npos.X), float32(npos.Y + npos.Height), 0)
			// 		debugSprite.Draw(pass, mat)
			// 	})
			// }

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
					mat.Translate(scale * (-bounds.W()/2), 15, 0) // TODO - 15 should come from the body height of the character (plus the font descent, or maybe half text line height)

					col := glitch.RGBA{1, 1, 1, 1}
					pass.SetLayer(glitch.DefaultLayer - 1) // TODO setup layers for world UI
					speech.Text.DrawColorMask(pass, mat, col)
				})
			}

			glitch.Clear(frame, glitch.RGBA{0, 0, 0, 0})

			tilemapPass.SetUniform("projection", camera.Camera.Projection)
			tilemapPass.SetUniform("view", camera.Camera.ViewSnapped)
			tilemapPass.Draw(frame)

			pass.SetUniform("projection", camera.Camera.Projection)
			pass.SetUniform("view", camera.Camera.ViewSnapped)
			pass.Draw(frame)

			windowPass.Clear()
			windowPass.SetUniform("projection", camera.Camera.Projection)

			vvx, vvy, _ := camera.Camera.View.GetTranslation()
			vsx, vsy, _ := camera.Camera.ViewSnapped.GetTranslation()
			mat := glitch.Mat4Ident
			mat.Translate(float32(vvx - vsx), float32(vvy - vsy), 0)
			windowPass.SetUniform("view", mat)

			frame.Draw(windowPass, glitch.Mat4Ident)
			windowPass.Draw(win)

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
