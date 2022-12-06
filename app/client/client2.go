package client

import (
	"time"
	// "math"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/glitch"

	"github.com/unitoftime/flow/ds"
	"github.com/unitoftime/flow/phy2"
	"github.com/unitoftime/flow/tile"
	"github.com/unitoftime/flow/net"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

// This is mostly for debug, but maybe its a good thing to track
type ServerTransform struct {
	phy2.Pos
	Handled bool
	ServerTick uint16
	PlayerTick uint16
}

type NetPos struct {
	InterpFrom, InterpTo phy2.Pos // The current interpolation values to be using
	Remaining, Total time.Duration

	ExtrapolatedPos, PreExtInterpTo phy2.Pos // The interpolation destination before the extrap value was added
}

func CreateClientSystems(world *ecs.World, sock *net.Socket, playerData *PlayerData, tilemap *tile.Tilemap) []ecs.System {
	clientSystems := []ecs.System{
		ecs.System{"ClientSendUpdate", func(dt time.Duration) {
			ClientSendUpdate(world, sock, playerData)
		}},
		ecs.System{"InterpolateSpritePositions", func(dt time.Duration) {
			// TODO - hack. We needed a way to create the transform component for other players (because we did a change which makes us set NextTransform over the wire instead of transform. So those were never being set

			playerId := playerData.Id()
			ecs.Map(world, func(id ecs.Id, serverTransform *ServerTransform) {
				pos, ok := ecs.Read[phy2.Pos](world, id)
				if !ok {
					pos = phy2.Pos{}
					ecs.Write(world, id, ecs.C(pos))
				}

				netPos, ok := ecs.Read[NetPos](world, id)

				if !serverTransform.Handled {
					// log.Print("New ServerTransform")
					serverTransform.Handled = true
					ecs.Write(world, id, ecs.C(serverTransform))

					netPos.InterpFrom = pos
					netPos.InterpTo = serverTransform.Pos
					netPos.PreExtInterpTo = netPos.InterpTo
					netPos.ExtrapolatedPos = netPos.InterpTo // This will be modified by client-side prediction

					netPos.Total = 64 * time.Millisecond // TODO! - This should be determined by the number of ticks that we are apart times the network tick time!!!

					// Idea: Add like a single frame of time to the total so that we get there a little bit slower. Ideally this would smooth the transitions between frames b/c we'd always lag behind.
					// netPos.Total = netPos.Total + 32 * time.Millisecond

					netPos.Remaining = netPos.Total

					// Extrapolate with trimmed player input buffer
					if id == playerId {
						inputBuffer := playerData.GetInputBuffer()

						//TODO! - Collider is wrong here
						collider, ok := ecs.Read[phy2.CircleCollider](world, playerId)
						if !ok { return } // Skip if player doesn't have a collider

						for i := range inputBuffer {
							// for ii := 0; ii < 4; ii++ { // TODO - 4 because we do %4 on send rate
							mmo.MoveCharacter(&inputBuffer[i].Input, &netPos.ExtrapolatedPos, &collider, tilemap, mmo.FixedTimeStep)
							// }
						}
					}
				}

				ecs.Write(world, id, ecs.C(netPos))
			})
		}},
	}

	physicsSystems := []ecs.System{
		ecs.System{"SetupColliders", func(dt time.Duration) {
			// Set the collider position
			ecs.Map2(world, func(id ecs.Id, netPos *NetPos, col *phy2.CircleCollider) {
				col.CenterX = netPos.InterpTo.X
				col.CenterY = netPos.InterpTo.Y
			})
		}},
		ecs.System{"CheckCollisions", func(dt time.Duration) {
			mmo.CheckCollisions(world)
		}},
	}
	clientSystems = append(clientSystems, physicsSystems...)
	return clientSystems
}

var everyOther int
func ClientSendUpdate(world *ecs.World, clientConn *net.Socket, playerData *PlayerData) {
	// TODO! - Not sure if this is okay
	// everyOther = (everyOther + 1) % 4
	// if everyOther != 0 {
	// 	return // skip
	// }

	playerId := playerData.Id()
	// if clientConn is closed for some reason, then we won't be able to send
	// TODO - With the atomic this fast enough?
	connected := clientConn.Connected.Load()
	if !connected { return } // Exit early because we are not connected

	input, ok := ecs.Read[mmo.Input](world, playerId)
	if !ok { return } // If we can't find the players input just exit early

	playerTick := playerData.AppendInputTick(input)

	compSlice := []ecs.Component{
		ecs.C(input),
	}

	// lastMsg := playerData.GetLastMessage()
	// // log.Print(lastMsg)
	// var messages []mmo.ChatMessage
	// if lastMsg != nil {
	// 	messages = []mmo.ChatMessage{
	// 		mmo.ChatMessage{
	// 			Username: "", // Note: Can't trust the username that the client sends
	// 			Message: lastMsg.Message,
	// 		},
	// 	}
	// }

	// If we can't find a speech, that's okay
	speech, speechFound := ecs.Read[mmo.Speech](world, playerId)
	if speechFound {
		if speech.HandleSent() {
			compSlice = append(compSlice, ecs.C(speech))
			ecs.Write(world, playerId, ecs.C(speech))
		}
	}

	// log.Print(messages)

	update := serdes.WorldUpdate{
		PlayerTick: playerTick,
		WorldData: map[ecs.Id][]ecs.Component{
			playerId: compSlice,
		},
		// Messages: messages,
	}
	// log.Print("ClientSendUpdate:", update)

	// log.Print(update)

	// Duplicate Sends to counter packet loss
	for i := 0; i < 4; i++ {
		err := clientConn.Send(update)
		if err != nil {
			log.Warn().Err(err).Msg("ClientSendUpdate")
		}
	}

	// ecs.Map2(world, func(id ecs.Id, _ *ClientOwned, input *phy2.Input) {
	// 	update := serdes.WorldUpdate{
	// 		WorldData: map[ecs.Id][]ecs.Component{
	// 			id: []ecs.Component{ecs.C(*input)},
	// 		},
	// 	}
	// 	log.Println("ClientSendUpdate:", update)

	// 	err := clientConn.Send(update)
	// 	if err != nil {
	// 		log.Println(err)
	// 	}
	// })
}

var AvgWorldUpdateTime time.Duration
func ClientReceive(sock *net.Socket, playerData *PlayerData, networkChannel chan serdes.WorldUpdate) error {
	// lastWorldUpdate := time.Now()
	bufLen := 100
	worldUpdateTimes := ds.NewRingBuffer[time.Duration](bufLen)
	for i := 0; i < bufLen; i++ {
		worldUpdateTimes.Add(4 * mmo.FixedTimeStep) // TODO! - hardcoded
	}

	for {
		msg, err := sock.Recv()
		if errors.Is(err, net.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			log.Warn().Err(err).Msg("ClientReceive NetworkErr")
			return err
		} else if errors.Is(err, net.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			log.Error().Err(err).Msg("ClientReceive SerdesErr")
			continue
		}
		if msg == nil { continue }

		switch t := msg.(type) {
		case serdes.WorldUpdate:
			// log.Print("Ticks: ", t.Tick, t.PlayerTick)
			// {
			// 	worldUpdateTimes.Add(time.Since(lastWorldUpdate))
			// 	lastWorldUpdate = time.Now()
			// 	buf := worldUpdateTimes.Buffer()
			// 	AvgWorldUpdateTime = 0
			// 	for i := range buf {
			// 		AvgWorldUpdateTime += buf[i]
			// 	}
			// 	AvgWorldUpdateTime = AvgWorldUpdateTime / time.Duration(len(buf))
			// 	// log.Print("AvgWorldUpdateTime: ", AvgWorldUpdateTime)
			// }

			// log.Print("Client-NewWorldUpdate")
			// playerData.SetTicks(t.Tick, t.PlayerTick)

			// Note: Because the client received this speech bubble update from the server, we will handle the HandleSent() so that the client doesn't try to resend it to the server.
			// This code just calls HandleSent() on the player's speech bubble if they just received their own speech bubble
			compSlice, ok := t.WorldData[playerData.Id()]
			if ok {
				newCompSlice := make([]ecs.Component, 0)
				// Pull out mmo.Speech for playerId
				for _, c := range compSlice {
					switch t := c.(type) {
					case ecs.CompBox[mmo.Speech]:
						msg := t.Get().Text
						log.Print("Client received a message for himself! ", msg)
						speech := mmo.Speech{
							Text: msg,
						}
						speech.HandleSent()
						// TODO - speech.HandleRender() - Would I ever use this to have the server send messages to the client?
						// compSlice[i] = ecs.C(speech)
						newCompSlice = append(newCompSlice, ecs.C(speech))
					case ecs.CompBox[mmo.Input]:
						// If the server sent us back our own input, we just want to drop it, because we own that component
						continue
					default:
						newCompSlice = append(newCompSlice, c)
					}
				}
				t.WorldData[playerData.Id()] = newCompSlice
			}

			for j, compSlice := range t.WorldData {
				for i, c := range compSlice {
					switch tt := c.(type) {
					case ecs.CompBox[phy2.Pos]:
						serverTransform := ServerTransform{
							Pos: tt.Get(),
							Handled: false,
							ServerTick: t.Tick,
							PlayerTick: t.PlayerTick,
						}
						compSlice[i] = ecs.C(serverTransform)
						// nextTransform := NextTransform{
						// 	PhyTrans: t.Get(),
						// 	Replayed: false,
						// }
						// serverTransform := ServerTransform(t.Get())
						// compSlice[i] = ecs.C(nextTransform)
						// compSlice = append(compSlice, ecs.C(serverTransform))
					}
				}
				t.WorldData[j] = compSlice

				// for i := range compSlice {
				// 	log.Printf("%T\n", compSlice[i])
				// }
			}

			networkChannel <- t
		case serdes.ClientLoginResp:
			log.Print("serdes.ClientLoginResp", t)
			// TODO this might be needed in the future if I want to write any data on login resp
			// ecs.Write(world, ecs.Id(t.Id), ecs.C(mmo.Body{}))
			// networkChannel <- serdes.WorldUpdate{
			// 	UserId: t.UserId,
			// 	WorldData: map[ecs.Id][]ecs.Component{
			// 		ecs.Id(t.Id): []ecs.Component{
			// 			ecs.C(mmo.Body{}),
			// 		},
			// 	},
			// }

			playerData.SetId(t.Id)

			networkChannel <- serdes.WorldUpdate{
				UserId: t.UserId,
				WorldData: map[ecs.Id][]ecs.Component{
					ecs.Id(t.Id): []ecs.Component{
						ecs.C(mmo.Input{}),
						ecs.C(phy2.Pos{}),
						ecs.C(Keybinds{
							Up: glitch.KeyW,
							Down: glitch.KeyS,
							Left: glitch.KeyA,
							Right: glitch.KeyD,
						}),
					},
				},
			}

		default:
			log.Error().Msg("Unknown message type")
		}
	}

	return nil
}
