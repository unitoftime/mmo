package client

//go:generate sh ase/export.sh

import (
	"os"
	"time"
	// "fmt"
	"embed"
	// "math"
	"strings"
	"flag"
	"crypto/tls"

	"github.com/zyedidia/generic/queue"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/glitch"
	"github.com/unitoftime/glitch/shaders"
	"github.com/unitoftime/glitch/ui"

	"github.com/unitoftime/flow/interp"
	"github.com/unitoftime/flow/asset"
	"github.com/unitoftime/flow/render"
	"github.com/unitoftime/flow/phy2"
	"github.com/unitoftime/flow/tile"
	"github.com/unitoftime/flow/net"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

//go:embed assets/*
var fs embed.FS

type Config struct {
	ProxyUri string
	Test bool
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
	// pixelArtShader, err := glitch.NewShader(shaders.PixelArtShader)
	// if err != nil { panic(err) }

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
	schedule := mmo.GetScheduler()
	schedule.AppendRender(renderSystems...)
	schedule.Run(&quit)
}

func runGame(win *glitch.Window, load *asset.Load, spritesheet *asset.Spritesheet, shader *glitch.Shader, atlas *glitch.Atlas) {
	pixelArtShader, err := glitch.NewShader(shaders.PixelArtShader)
	if err != nil { panic(err) }

	world := ecs.NewWorld()
	networkChannel := make(chan serdes.WorldUpdate, 1024) // TODO - arbitrary 1024

	// This is the player's ID, by default we set this to invalid
	playerData := NewPlayerData()

	// TODO - Do this for local testing (Right now I'm doing insecure skip verify)
	// Ref: https://github.com/jcbsmpsn/golang-https-example
	// cert, err := os.ReadFile("cert.pem")
	// if err != nil {
	// 	panic(err)
	// }
	// caCertPool := x509.NewCertPool()
	// caCertPool.AppendCertsFromPEM(caCert)
	// tlsConfig := &tls.Config{
	// 	RootCAs: caCertPool,
	// }

	proxyNet := net.Config{
		Url: globalConfig.ProxyUri,
		Serdes: serdes.New(),
		TlsConfig: &tls.Config{
			InsecureSkipVerify: globalConfig.Test, // If test mode, then we don't care about the cert
		},
		ReconnectHandler: func(sock *net.Socket) error {
			return ClientReceive(sock, playerData, networkChannel)
		},
	}

	sock, err := proxyNet.Dial()
	if err != nil {
		panic(err)
	}

	// Note: This requires a system to update the framebuffer if the window is resized. The system should essentially recreate the framebuffer with the new dimensions, This might be a good target for the framebuffer callback, but for now I'm just going to poll win.Bounds
	renderBounds := win.Bounds()
	frame := glitch.NewFrame(renderBounds, true)

	windowPass := glitch.NewRenderPass(pixelArtShader)

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

	debugMode := false

	textInputMode := false

	screenCamera := glitch.NewCameraOrtho()
	screenCamera.SetOrtho2D(win.Bounds())
	screenCamera.SetView2D(0, 0, 1.0, 1.0)
	group := ui.NewGroup(win, screenCamera, atlas)

	camera := render.NewCamera(win.Bounds(), 0, 0)
	camera.Zoom = 2.0

	updateQueue := queue.New[serdes.WorldUpdate]()

	quit := ecs.Signal{}
	quit.Set(false)

	inputSystems := []ecs.System{
		ClientPollNetworkSystem(networkChannel, updateQueue),
		ClientPullFromUpdateQueue(world, updateQueue, playerData),
		ecs.System{"ManageEntityTimeout", func(dt time.Duration) {
			timeout := 5 * time.Second
			now := time.Now()
			ecs.Map(world, func(id ecs.Id, lastUpdate *LastUpdate) {
				if now.Sub(lastUpdate.Time) > timeout {
					ecs.Delete(world, id)
				}
			})
		}},
		ecs.System{"BodySetup", func(dt time.Duration) {
			ecs.Map(world, func(id ecs.Id, body *mmo.Body) {
				// TODO - is there a way to not have to poll these each frame?
				// Body to animation
				_, ok := ecs.Read[Animation](world, id)
				if !ok {
					ecs.Write(world, id,
						ecs.C(NewAnimation(load, spritesheet, *body)),
					)
				}

				// Body to collider
				_, ok = ecs.Read[phy2.CircleCollider](world, id)
				if !ok {
					// TODO - hardcoded here and in network.go - Centralize character creation
					// TODO - arbitrary collider radius 6
					collider := phy2.NewCircleCollider(6)
					collider.Layer = mmo.BodyLayer
					collider.HitLayer = mmo.BodyLayer
					ecs.Write(world, id,
						ecs.C(collider),
						ecs.C(phy2.NewColliderCache()),
					)
				}
			})

			// Tile objects?
			ecs.Map(world, func(id ecs.Id, body *mmo.TileObject) {
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

			const minZoom = 2
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

				CaptureInput(win, world)
			} else {
				// Clear current inputs
				ecs.Map2(world, func(id ecs.Id, keybinds *Keybinds, input *mmo.Input) {
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
			// playerId := playerData.Id()
			// playerPhyT, _ := ecs.Read[phy2.Transform](world, playerId)

			ecs.Map2(world, func(id ecs.Id, pos *phy2.Pos, nextT *NextTransform) {
				nextT.Remaining -= dt

				interpFactor := 1 - (nextT.Remaining.Seconds() / nextT.Total.Seconds())
				if interpFactor > 1 { // TODO - can I prevent this from going above, makes it stop for a second/frame
					interpFactor = 1
				}

				// old := *phyT
				// phyT.X = interp.Linear.Float64(nextT.InterpFrom.X, nextT.InterpTo.X, interpFactor)
				// phyT.Y = interp.Linear.Float64(nextT.InterpFrom.Y, nextT.InterpTo.Y, interpFactor)
				pos.X = interp.Linear.Float64(nextT.InterpFrom.X, nextT.ExtrapolationOffset.X, interpFactor)
				pos.Y = interp.Linear.Float64(nextT.InterpFrom.Y, nextT.ExtrapolationOffset.Y, interpFactor)

				// log.Print("Trans: ", interpFactor, phyT.Sub(&playerPhyT), phyT.Sub(&old))


				iFactor := 0.2
				nextT.ExtrapolationOffset.X = interp.Linear.Float64(nextT.ExtrapolationOffset.X, nextT.Extrapolation.X, iFactor)
				nextT.ExtrapolationOffset.Y = interp.Linear.Float64(nextT.ExtrapolationOffset.Y, nextT.Extrapolation.Y, iFactor)
			})

			minAnim := 2.0
			ecs.Map4(world, func(id ecs.Id, input *mmo.Input, anim *Animation, pos *phy2.Pos, nextT *NextTransform) {
				if input.Left && !input.Right {
					anim.Direction = "left"
					anim.SetAnimation("run_left")
				} else if input.Right && !input.Left {
					anim.Direction = "right"
					anim.SetAnimation("run_right")
				} else if input.Up || input.Down {
					anim.SetAnimation("run_" + anim.Direction)
				} else {
					// if phyT.DistanceTo(&nextT.PhyTrans) > minAnim {
					// 	return // Don't set idle because we are still interpolating to our destination
					// }
					// next := nextT.First()
					next, ok := nextT.PeekLast()
					if !ok { return } // TODO - notsure what to do here
					if pos.Sub(next.Pos).Len() > minAnim {
						return // Don't set idle because we are still interpolating to our destination
					}

					if input.Left && input.Right {
						anim.SetAnimation("idle_" + anim.Direction)
					} else {
						anim.SetAnimation("idle_" + anim.Direction)
					}
				}
			})
		}},
	}

	physicsSystems := CreateClientSystems(world, sock, playerData, tilemap)

	panelSprite, err := spritesheet.GetNinePanel("ui_panel0.png", glitch.R(2, 2, 2, 2))
	if err != nil { panic(err) }
	panelSprite.Scale = 8
	textInputString := ""

	debugSprite, err := spritesheet.Get("ui_panel0.png")
	if err != nil { panic(err) }

	renderSystems := []ecs.System{
		ecs.System{"UpdateFramebuffer", func(dt time.Duration) {
			renderBounds := win.Bounds()
			// TODO - how to determine 16?
			renderBounds = renderBounds.Pad(glitch.R(16,16,16,16)) // Pad out by 16 pixel so that camera can drift inside pixels
			if frame.Bounds() != renderBounds {
				frame = glitch.NewFrame(renderBounds, true)
				// log.Print("recreating fbo: ", frame.Bounds(), renderBounds, win.Bounds())
			}
		}},
		ecs.System{"UpdateCamera", func(dt time.Duration) {
			transform, ok := ecs.Read[phy2.Pos](world, playerData.Id())
			if ok {
				// log.Print("Update Camera", transform)
				// sprite := comp[1].(*render.Sprite)
				// camera.Position = sprite.Position
				camera.Position = glitch.Vec2{float32(transform.X), float32(transform.Y)}
			}

			// ecs.Map2(world, func(id ecs.Id, _ *mmo.ClientOwned, transform *phy2.Transform) {
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

			ecs.Map2(world, func(id ecs.Id, sprite *render.Sprite, pos *phy2.Pos) {
				sprite.Draw(pass, pos)
			})

			PlayAnimations(pass, world, dt)

			// Debug. Draw neworking position buffer
			if debugMode {
				pass.SetLayer(0)
				ecs.Map2(world, func(id ecs.Id, pos *phy2.Pos, nt *NextTransform) {

					// npos := nt.PhyTrans
					// npos := nt.Last()
					// mat := glitch.Mat4Ident
					// mat.Scale(0.5, 0.5, 1.0).Translate(float32(npos.X), float32(npos.Y + npos.Height), 0)
					// debugSprite.Draw(pass, mat)

					// Interp Replay buffer
					nt.Map(func(t ServerTransform) {
						mat := glitch.Mat4Ident
						mat.Scale(0.5, 0.5, 1.0).Translate(float32(t.X), float32(t.Y), 0)
						debugSprite.DrawColorMask(pass, mat, glitch.RGBA{0, 1, 0, 0.5})
					})

					mat := glitch.Mat4Ident
					mat.Scale(0.5, 0.5, 1.0).Translate(float32(nt.ExtrapolationOffset.X), float32(nt.ExtrapolationOffset.Y), 0)
					debugSprite.DrawColorMask(pass, mat, glitch.RGBA{0, 0, 0, 1})

					mat = glitch.Mat4Ident
					// mat.Scale(0.5, 0.5, 1.0).Translate(float32(nt.InterpFrom.X + nt.Extrapolation.X), float32(nt.InterpFrom.Y + nt.Extrapolation.Y + nt.Extrapolation.Height), 0)
					mat.Scale(0.5, 0.5, 1.0).Translate(float32(nt.Extrapolation.X), float32(nt.Extrapolation.Y), 0)
					debugSprite.DrawColorMask(pass, mat, glitch.RGBA{1, 1, 1, 1})

					mat = glitch.Mat4Ident
					mat.Scale(0.5, 0.5, 1.0).Translate(float32(nt.InterpFrom.X), float32(nt.InterpFrom.Y), 0)
					debugSprite.DrawColorMask(pass, mat, glitch.RGBA{1, 0, 0, 1})

					mat = glitch.Mat4Ident
					mat.Scale(0.5, 0.5, 1.0).Translate(float32(nt.InterpTo.X), float32(nt.InterpTo.Y), 0)
					debugSprite.DrawColorMask(pass, mat, glitch.RGBA{0, 1, 0, 1})
				})

				// Last Server Position
				// ecs.Map(world, func(id ecs.Id, t *ServerTransform) {
				// 	mat := glitch.Mat4Ident
				// 	mat.Scale(0.5, 0.5, 1.0).Translate(float32(t.X), float32(t.Y + t.Height), 0)
				// 	debugSprite.DrawColorMask(pass, mat, glitch.RGBA{1, 0, 0, 1})
				// })

				inputBuffer := playerData.GetInputBuffer()
				tBuffer, _ := ecs.Read[NextTransform](world, playerData.Id())
				collider, _ := ecs.Read[phy2.CircleCollider](world, playerData.Id())
				extPos := tBuffer.PreExtInterpTo.Pos
				for i := range inputBuffer {
					// for ii := 0; ii < 4; ii++ { // TODO - 4 because we do %4 on send rate
						mmo.MoveCharacter(&inputBuffer[i].Input, &extPos, &collider, tilemap, mmo.FixedTimeStep)
					// }

					mat := glitch.Mat4Ident
					mat.Scale(0.25, 0.25, 1.0).Translate(float32(extPos.X), float32(extPos.Y), 0)
					debugSprite.DrawColorMask(pass, mat, glitch.RGBA{1, 1, 1, 1})
				}
			}

			pass.SetLayer(glitch.DefaultLayer)

			// Draw speech bubbles
			{
				// TODO - move to physics system
				{
					commandList := make([]func(), 0)
					ecs.Map(world, func(id ecs.Id, speech *mmo.Speech) {
						if speech.HandleRender() {
							commandList = append(commandList,
								func() {
									// TODO - combine SpeechRender component with otherone in mmo.SetSpeech()
									ecs.Write(world, id, ecs.C(SpeechRender{
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

				ecs.Map2(world, func(id ecs.Id, speech *SpeechRender, pos *phy2.Pos) {

					if speech.RemainingDuration < 0 { return } // Skip the display duration has ended
					speech.RemainingDuration -= dt

					scale := float32(0.4)
					mat := glitch.Mat4Ident
					mat.Scale(scale, scale, 1.0).Translate(float32(pos.X), float32(pos.Y), 0)
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

			mat := glitch.Mat4Ident
			vvx, vvy, _ := camera.Camera.View.GetTranslation()
			vsx, vsy, _ := camera.Camera.ViewSnapped.GetTranslation()
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
					rtt := playerData.RoundTripTimes()
					rttPoints := make([]glitch.Vec2, len(rtt))
					for i := range rtt {
						rttPoints[i] = glitch.Vec2{
							float32(i),
							float32(1000 * rtt[i].Seconds()),
						}
					}
					rttRect := connectedRect.Anchor(glitch.R(0, 0, 200, 100), glitch.Vec2{1, 1})//.Moved(glitch.Vec2{0, connectedRect.H()})
					if debugMode {
						group.SetColor(glitch.RGBA{0, 0, 1, 1})
						group.LineGraph(rttRect, rttPoints)
					}
				} else {
					group.SetColor(glitch.RGBA{1, 0, 0, 1})
					group.FixedText("Disconnected", connectedRect, glitch.Vec2{1, 0}, textScale)
				}

				if !textInputMode && win.JustPressed(glitch.KeyEnter) {
					textInputMode = true
				} else if !textInputMode && win.JustPressed(glitch.KeySlash) {
					textInputMode = true
					textInputString = "/"
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
						if strings.HasPrefix(textInputString, "/") {
							if strings.HasPrefix(textInputString, "/debug") {
								debugMode = !debugMode
							}
						} else {
							// Write the player's speech bubble
							SetSpeech(world, atlas, playerData.Id(), textInputString)
						}

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

	schedule := mmo.GetScheduler()

	// physicsSystems = append(physicsSystems, ecs.System{"UpdateWindow", func(dt time.Duration) {
	// 	syslog := schedule.Syslog()
	// 	for i := range syslog {
	// 		log.Print(syslog[i])
	// 	}
	// }})
	schedule.AppendInput(inputSystems...)
	schedule.AppendPhysics(physicsSystems...)
	schedule.AppendRender(renderSystems...)

	schedule.Run(&quit)
	// ecs.RunGame(inputSystems, physicsSystems, renderSystems, &quit)
	log.Print("Finished ecs.RunGame")

	// TODO - I'm not sure if this is the proper way to close because `ClientReceive` is still reading, so closing here will cause that to fail
	sock.Close()
}
