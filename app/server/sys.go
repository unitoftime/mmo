package server

import (
	"time"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/tile"
	"github.com/unitoftime/flow/phy2"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

// Represents a logged in user on the server
type User struct {
	// Name string // TODO - remove and put into a component called "DisplayName"
	Id uint64
	ProxyId uint64
}

// This is the tick that the client says they are on
type ClientTick struct {
	Tick uint16 // This is the tick that the player is currently on
}

func CreateServerSystems(world *ecs.World, server *Server, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList, tilemap *tile.Tilemap) []ecs.System {
	serverSystems := []ecs.System{
		CreatePollNetworkSystem(world, networkChannel),
	}

	// serverSystems = append(serverSystems,
	// 	CreatePhysicsSystems(world)...)
	serverSystems = append(serverSystems,
		ecs.System{"MoveCharacters", func(dt time.Duration) {
			ecs.Map3(world, func(id ecs.Id, input *mmo.Input, pos *phy2.Pos, collider *phy2.CircleCollider) {
				mmo.MoveCharacter(input, pos, collider, tilemap, dt)
			})
		}},
		ecs.System{"CheckCollisions", func(dt time.Duration) {
			// Set the collider position
			ecs.Map2(world, func(id ecs.Id, pos *phy2.Pos, col *phy2.CircleCollider) {
				col.CenterX = pos.X
				col.CenterY = pos.Y
			})

			mmo.CheckCollisions(world)
		}},
	)

	serverSystems = append(serverSystems, []ecs.System{
		ecs.System{"ServerSendUpdate", func(dt time.Duration) {
			ServerSendUpdate(world, server, deleteList)
		}},
	}...)

	return serverSystems
}
