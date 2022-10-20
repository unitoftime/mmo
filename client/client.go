package client

import (
	"time"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/flow/render"
	"github.com/unitoftime/glitch"
	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/game"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/mmo/mnet"
)

func CreateClientSystems(world *ecs.World, sock *mnet.Socket, playerData *mmo.PlayerData) []ecs.System {
	clientSystems := []ecs.System{
		ecs.System{"ClientSendUpdate", func(dt time.Duration) {
			ClientSendUpdate(world, sock, playerData)
		}},
	}

	physicsSystems := mmo.CreatePhysicsSystems(world)
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
			// Note: Because the client received this speech bubble update from the server, we will handle the HandleSent() so that the client doesn't try to resend it to the server.
			// This code just calls HandleSent() on the player's speech bubble if they just received their own speech bubble
			compSlice, ok := t.WorldData[playerData.Id()]
			if ok {
				for i, c := range compSlice {
					switch t := c.(type) {
					case ecs.CompBox[game.Speech]:
						msg := t.Get().Text
						log.Print("Client received a message for himself! ", msg)
						speech := game.Speech{
							Text: msg,
						}
						speech.HandleSent()
						// TODO - speech.HandleRender() - Would I ever use this to have the server send messages to the client?x
						compSlice[i] = ecs.C(speech)
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
