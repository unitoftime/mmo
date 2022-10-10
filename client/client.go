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

	update := serdes.WorldUpdate{
		WorldData: map[ecs.Id][]ecs.Component{
			playerId: []ecs.Component{ecs.C(input)},
		},
	}
	// log.Print("ClientSendUpdate:", update)

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
