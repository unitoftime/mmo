package client

import (
	"time"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/flow/render"
	"github.com/unitoftime/flow/interp"
	"github.com/unitoftime/flow/tile"
	"github.com/unitoftime/glitch"
	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/game"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/mmo/mnet"
)

type NextTransform struct {
	PhyTrans physics.Transform
	Replayed bool
}
// type LastTransform physics.Transform
// TODO - do a full fledged transform buffer
// type TransformBuffer struct {
// 	buffer []physics.Transform
// }

func CreateClientSystems(world *ecs.World, sock *mnet.Socket, playerData *mmo.PlayerData, tilemap *tile.Tilemap) []ecs.System {
	clientSystems := []ecs.System{
		ecs.System{"ClientSendUpdate", func(dt time.Duration) {
			ClientSendUpdate(world, sock, playerData)
		}},
		ecs.System{"ReplayInputBuffer", func(dt time.Duration) {
			// Replays the remaining input buffer to try and guesstimate the player's position
			inputBuffer := playerData.GetInputBuffer()
			playerId := playerData.Id()
			transform, ok := ecs.Read[NextTransform](world, playerId)
			if !ok { return } // Skip if player doesn't have a transform
			collider, ok := ecs.Read[physics.CircleCollider](world, playerId)
			if !ok { return } // Skip if player doesn't have a collider
			if !transform.Replayed {
				for i := range inputBuffer {
					mmo.MoveCharacter(&inputBuffer[i], &transform.PhyTrans, &collider, tilemap, ecs.FixedTimeStep)
				}
				transform.Replayed = true
				ecs.Write(world, playerId, ecs.C(transform))
			}
		}},
		// Note: This must be after ReplayInputBuffer because if a replay happens, then we want to interpolate halfway there
		ecs.System{"InterpolateSpritePositions", func(dt time.Duration) {
			// TODO - hack. We needed a way to create the transform component for other players (because we did a change which makes us set NextTransform over the wire instead of transform. So those were never being set
			// Logic: If has next transform, but doesn't have transform, then add transform
			ecs.Map(world, func(id ecs.Id, nextT *NextTransform) {
				_, ok := ecs.Read[physics.Transform](world, id)
				if !ok {
					ecs.Write(world, id, ecs.C(physics.Transform{}))
				}
			})

			// This interpolates the transform position based on what the server just said it was
			const maxInterp float64 = 5 * 16.0
			ecs.Map2(world, func(id ecs.Id, phyT *physics.Transform, nextT *NextTransform) {
				// Snap, rather than interpolate if the distance is large enough
				if phyT.DistanceTo(&nextT.PhyTrans) > maxInterp {
					phyT.X = nextT.PhyTrans.X
					phyT.Y = nextT.PhyTrans.Y
					return
				}

				interpFactor := 0.1
				// interpFactor := 1.0
				phyT.X = interp.Linear.Float64(phyT.X, nextT.PhyTrans.X, interpFactor)
				phyT.Y = interp.Linear.Float64(phyT.Y, nextT.PhyTrans.Y, interpFactor)
			})
		}},
	}

	physicsSystems := []ecs.System{
		ecs.System{"MoveCharacters", func(dt time.Duration) {
			// TODO - Note: Keybinds is only included so that clients don't simulate another client's input, I should isolate these another way
			ecs.Map4(world, func(id ecs.Id, input *physics.Input, keybinds *render.Keybinds, nextTrans *NextTransform, collider *physics.CircleCollider) {
			// ecs.Map3(world, func(id ecs.Id, input *physics.Input, nextTrans *NextTransform, collider *physics.CircleCollider) {
				mmo.MoveCharacter(input, &nextTrans.PhyTrans, collider, tilemap, dt)
			})
		}},
		ecs.System{"SetupColliders", func(dt time.Duration) {
			// Set the collider position
			ecs.Map2(world, func(id ecs.Id, transform *NextTransform, col *physics.CircleCollider) {
				col.CenterX = transform.PhyTrans.X
				col.CenterY = transform.PhyTrans.Y
			})
		}},
		ecs.System{"CheckCollisions", func(dt time.Duration) {
			mmo.CheckCollisions(world)
		}},
	}
	clientSystems = append(clientSystems, physicsSystems...)
	return clientSystems
}

func ClientSendUpdate(world *ecs.World, clientConn *mnet.Socket, playerData *mmo.PlayerData) {
	playerId := playerData.Id()
	// if clientConn is closed for some reason, then we won't be able to send
	// TODO - With the atomic this fast enough?
	connected := clientConn.Connected.Load()
	if !connected { return } // Exit early because we are not connected

	input, ok := ecs.Read[physics.Input](world, playerId)
	if !ok { return } // If we can't find the players input just exit early

	playerTick := playerData.AppendInputTick(input)

	compSlice := []ecs.Component{
		ecs.C(input),
	}

	// lastMsg := playerData.GetLastMessage()
	// // log.Print(lastMsg)
	// var messages []game.ChatMessage
	// if lastMsg != nil {
	// 	messages = []game.ChatMessage{
	// 		game.ChatMessage{
	// 			Username: "", // Note: Can't trust the username that the client sends
	// 			Message: lastMsg.Message,
	// 		},
	// 	}
	// }

	// If we can't find a speech, that's okay
	speech, speechFound := ecs.Read[game.Speech](world, playerId)
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

	err := clientConn.Send(update)
	if err != nil {
		log.Warn().Err(err).Msg("ClientSendUpdate")
	}

	// ecs.Map2(world, func(id ecs.Id, _ *ClientOwned, input *physics.Input) {
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

func ClientReceive(sock *mnet.Socket, playerData *mmo.PlayerData, networkChannel chan serdes.WorldUpdate) error {
	for {
		msg, err := sock.Recv()
		if errors.Is(err, mnet.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			log.Warn().Err(err).Msg("ClientReceive NetworkErr")
			return err
		} else if errors.Is(err, mnet.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			log.Error().Err(err).Msg("ClientReceive SerdesErr")
			continue
		}
		if msg == nil { continue }

		switch t := msg.(type) {
		case serdes.WorldUpdate:
			playerData.SetTicks(t.Tick, t.PlayerTick)

			// Note: Because the client received this speech bubble update from the server, we will handle the HandleSent() so that the client doesn't try to resend it to the server.
			// This code just calls HandleSent() on the player's speech bubble if they just received their own speech bubble
			compSlice, ok := t.WorldData[playerData.Id()]
			if ok {
				newCompSlice := make([]ecs.Component, 0)
				// Pull out game.Speech for playerId
				for _, c := range compSlice {
					switch t := c.(type) {
					case ecs.CompBox[game.Speech]:
						msg := t.Get().Text
						log.Print("Client received a message for himself! ", msg)
						speech := game.Speech{
							Text: msg,
						}
						speech.HandleSent()
						// TODO - speech.HandleRender() - Would I ever use this to have the server send messages to the client?
						// compSlice[i] = ecs.C(speech)
						newCompSlice = append(newCompSlice, ecs.C(speech))
					case ecs.CompBox[physics.Input]:
						// If the server sent us back our own input, we just want to drop it, because we own that component
						continue
					default:
						newCompSlice = append(newCompSlice, c)
					}
				}
				t.WorldData[playerData.Id()] = newCompSlice
			}

			for _, compSlice := range t.WorldData {
				for i, c := range compSlice {
					switch t := c.(type) {
					case ecs.CompBox[physics.Transform]:
						nextTransform := NextTransform{
							PhyTrans: t.Get(),
							Replayed: false,
						}
						compSlice[i] = ecs.C(nextTransform)
					}
				}
			}

			networkChannel <- t
		case serdes.ClientLoginResp:
			log.Print("serdes.ClientLoginResp", t)
			// TODO this might be needed in the future if I want to write any data on login resp
			// ecs.Write(world, ecs.Id(t.Id), ecs.C(game.Body{}))
			// networkChannel <- serdes.WorldUpdate{
			// 	UserId: t.UserId,
			// 	WorldData: map[ecs.Id][]ecs.Component{
			// 		ecs.Id(t.Id): []ecs.Component{
			// 			ecs.C(game.Body{}),
			// 		},
			// 	},
			// }

			playerData.SetId(t.Id)

			networkChannel <- serdes.WorldUpdate{
				UserId: t.UserId,
				WorldData: map[ecs.Id][]ecs.Component{
					ecs.Id(t.Id): []ecs.Component{
						ecs.C(physics.Input{}),
						ecs.C(physics.Transform{}),
						ecs.C(render.Keybinds{
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
