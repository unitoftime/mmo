package mmo

import (
	// "log"
	"time"
	"math"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/tile"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/flow/pgen"
	"github.com/unitoftime/mmo/serdes"
)

var seed int64 = 12345
var mapSize int = 100
var tileSize int = 16

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
	Name string // TODO - remove and put into a component called "DisplayName"
	Id uint64
}

type ClientOwned struct {
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

func CreateClientSystems(world *ecs.World, clientConn ClientConn) []ecs.System {
	clientSystems := []ecs.System{
		ecs.System{"ClientSendUpdate", func(dt time.Duration) {
			ClientSendUpdate(world, clientConn)
		}},
	}

	physicsSystems := CreatePhysicsSystems(world)
	clientSystems = append(clientSystems, physicsSystems...)
	return clientSystems
}

func CreateServerSystems(world *ecs.World, serverConn ServerConn, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList) []ecs.System {
	serverSystems := []ecs.System{
		CreatePollNetworkSystem(world, networkChannel),
	}

	serverSystems = append(serverSystems,
		CreatePhysicsSystems(world)...)

	serverSystems = append(serverSystems, []ecs.System{
		ecs.System{"ServerSendUpdate", func(dt time.Duration) {
			ServerSendUpdate(world, serverConn, deleteList)
		}},
	}...)

	return serverSystems
}

func CreatePollNetworkSystem(world *ecs.World, networkChannel chan serdes.WorldUpdate) ecs.System {
	sys := ecs.System{"PollNetworkChannel", func(dt time.Duration) {

	MainLoop:
		for {
			select {
			case update := <-networkChannel:
				// log.Println(update)
				// ecs.Write(engine, update.Id, update.Component)
				for id, compList := range update.WorldData {
					// log.Println("CompList:", id, compList)
					ecs.Write(world, id, compList...)

//TODO - Forcing this to fail: Note: We removed position from sprite with the plan to make an networkPosition be the thing that comes off the network. Then have a system that interps that into the current transform

					// for i := range compList {
					// 	// fmt.Printf("HERE %T", compList[i])
					// 	// log.Println(compList[i])

					// 	ecs.Write(engine, id, compList[i])
					// }
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

