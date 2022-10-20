package mmo

import (
	"time"
	"math"
	"sync"

	// "github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/tile"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/flow/pgen"
	// "github.com/unitoftime/mmo/game"
	"github.com/unitoftime/mmo/serdes"
)

var seed int64 = 12345
var mapSize int = 100
var tileSize int = 16

// This represents global player data on the client
type PlayerData struct {
	mu sync.RWMutex
	id ecs.Id
	lastMessage string
}
func NewPlayerData() *PlayerData {
	return &PlayerData{
		id: ecs.InvalidEntity,
	}
}

func (p *PlayerData) Id() ecs.Id {
	p.mu.RLock()
	ret := p.id
	p.mu.RUnlock()
	return ret
}

func (p *PlayerData) SetId(id ecs.Id) {
	p.mu.Lock()
	p.id = id
	p.mu.Unlock()
}

// Returns the message as sent to the server
// TODO - if the player sends another message fast enough, it could blank out their first message
// func (p *PlayerData) SendMessage(msg string) string {
// 	msg = serdes.FilterChat(msg)
// 	p.lastMessage = msg
// 	return msg
// }

// // Returns the last message and clears the last message buffer, returns nil if no new message
// func (p *PlayerData) GetLastMessage() *game.ChatMessage {
// 	if p.lastMessage == "" {
// 		return nil
// 	}

// 	msg := p.lastMessage
// 	p.lastMessage = ""
// 	return &game.ChatMessage{
// 		// Username: nil, // TODO - return username?
// 		Message: msg,
// 	}
// }

func SpawnPoint() physics.Transform {
	spawnPoint := physics.Transform{
		X: float64(tileSize*mapSize/2),
		Y: float64(tileSize*mapSize/2),
		Height: 0,
	}
	return spawnPoint
}

func LoadGame(world *ecs.World) *tile.Tilemap {
	// Create Tilemap
	tmap := CreateTilemap(seed, mapSize, tileSize)

	return tmap
}

// Represents a logged in user on the server
type User struct {
	// Name string // TODO - remove and put into a component called "DisplayName"
	Id uint64
	ProxyId uint64
}

const (
	GrassTile tile.TileType = iota
	DirtTile
	WaterTile
)

func CreateTilemap(seed int64, mapSize, tileSize int) *tile.Tilemap {
	octaves := []pgen.Octave{
		pgen.Octave{0.01, 0.6},
		pgen.Octave{0.05, 0.3},
		pgen.Octave{0.1, 0.07},
		pgen.Octave{0.2, 0.02},
		pgen.Octave{0.4, 0.01},
	}
	exponent := 0.8
	terrain := pgen.NewNoiseMap(seed, octaves, exponent)

	waterLevel := 0.5
	beachLevel := waterLevel + 0.1

	islandExponent := 2.0
	tiles := make([][]tile.Tile, mapSize, mapSize)
	for x := range tiles {
		tiles[x] = make([]tile.Tile, mapSize, mapSize)
		for y := range tiles[x] {

			height := terrain.Get(x, y)

			// Modify height to represent an island
			{
				dx := float64(x)/float64(mapSize) - 0.5
				dy := float64(y)/float64(mapSize) - 0.5
				d := math.Sqrt(dx * dx + dy * dy) * 2
				d = math.Pow(d, islandExponent)
				height = (1 - d + height) / 2
			}

			if height < waterLevel {
				tiles[x][y] = tile.Tile{WaterTile, 0, ecs.InvalidEntity}
			} else if height < beachLevel {
				tiles[x][y] = tile.Tile{DirtTile, 0, ecs.InvalidEntity}
			} else {
				tiles[x][y] = tile.Tile{GrassTile, 0, ecs.InvalidEntity}
			}
		}
	}
	tmap := tile.New(tiles, [2]int{tileSize, tileSize}, tile.FlatRectMath{})

	return tmap
}

func CreatePhysicsSystems(world *ecs.World) []ecs.System {
	physicsSystems := []ecs.System{
		ecs.System{"HandleInput", func(dt time.Duration) {
			physics.HandleInput(world, dt)
		}},
	}
	return physicsSystems
}

func CreateServerSystems(world *ecs.World, server *Server, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList) []ecs.System {
	serverSystems := []ecs.System{
		CreatePollNetworkSystem(world, networkChannel),
	}

	serverSystems = append(serverSystems,
		CreatePhysicsSystems(world)...)

	serverSystems = append(serverSystems, []ecs.System{
		ecs.System{"ServerSendUpdate", func(dt time.Duration) {
			ServerSendUpdate(world, server, deleteList)
		}},
	}...)

	return serverSystems
}

type LastUpdate struct {
	Time time.Time
}

func CreatePollNetworkSystem(world *ecs.World, networkChannel chan serdes.WorldUpdate) ecs.System {
	sys := ecs.System{"PollNetworkChannel", func(dt time.Duration) {

	MainLoop:
		for {
			select {
			case update := <-networkChannel:
				for id, compList := range update.WorldData {
					compList = append(compList, ecs.C(LastUpdate{time.Now()}))
					ecs.Write(world, id, compList...)

// TODO - Do I still need interpolation like this?
// TODO - Forcing this to fail: Note: We removed position from sprite with the plan to make an networkPosition be the thing that comes off the network. Then have a system that interps that into the current transform
				}

				// Delete all the entities in the deleteList
				if update.Delete != nil {
					for _, id := range update.Delete {
						ecs.Delete(world, id)
					}
				}

			default:
				break MainLoop
			}
		}
	}}

	return sys
}

