package mmo

import (
	"time"
	"math"
	"net"

	"go.nanomsg.org/mangos/v3"

	"github.com/jstewart7/mmo/engine/ecs"
	"github.com/jstewart7/mmo/engine/tilemap"
	"github.com/jstewart7/mmo/engine/physics"
	"github.com/jstewart7/mmo/engine/pgen"
	"github.com/jstewart7/mmo/serdes"
)

var seed int64 = 12345
var mapSize int = 100
var tileSize int = 16

func SpawnPoint() physics.Transform {
	spawnPoint := physics.Transform{
		float64(tileSize*mapSize/2),
		float64(tileSize*mapSize/2)}
	return spawnPoint
}

func LoadGame(engine *ecs.Engine) *tilemap.Tilemap {
	// Create Tilemap
	tmap := CreateTilemap(seed, mapSize, tileSize)

	return tmap
}

// Represents a logged in user on the server
type User struct {
	Name string // TODO - remove and put into a component called "DisplayName"
	Id uint64
}
func (t *User) ComponentSet(val interface{}) { *t = val.(User) }

type Body struct {
}
func (t *Body) ComponentSet(val interface{}) { *t = val.(Body) }

type ClientOwned struct {
}
func (t *ClientOwned) ComponentSet(val interface{}) { *t = val.(ClientOwned) }

const (
	GrassTile tilemap.TileType = iota
	DirtTile
	WaterTile
)

func CreateTilemap(seed int64, mapSize, tileSize int) *tilemap.Tilemap {
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
	tiles := make([][]tilemap.Tile, mapSize, mapSize)
	for x := range tiles {
		tiles[x] = make([]tilemap.Tile, mapSize, mapSize)
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
				tiles[x][y] = tilemap.Tile{WaterTile}
			} else if height < beachLevel {
				tiles[x][y] = tilemap.Tile{DirtTile}
			} else {
				tiles[x][y] = tilemap.Tile{GrassTile}
			}
		}
	}
	tmap := tilemap.New(tiles, tileSize)

	return tmap
}

func CreatePhysicsSystems(engine *ecs.Engine) []ecs.System {
	physicsSystems := []ecs.System{
		ecs.System{"HandleInput", func(dt time.Duration) {
			physics.HandleInput(engine, dt)
		}},
	}
	return physicsSystems
}

func CreateClientSystems(engine *ecs.Engine, conn net.Conn) []ecs.System {
	clientSystems := []ecs.System{
		ecs.System{"ClientSendUpdate", func(dt time.Duration) {
			ClientSendUpdate(engine, conn)
		}},
	}

	physicsSystems := CreatePhysicsSystems(engine)
	clientSystems = append(clientSystems, physicsSystems...)
	return clientSystems
}

func CreateServerSystems(engine *ecs.Engine, sock mangos.Socket, networkChannel chan serdes.WorldUpdate) []ecs.System {
	serverSystems := []ecs.System{
		CreatePollNetworkSystem(engine, networkChannel),
	}

	serverSystems = append(serverSystems,
		CreatePhysicsSystems(engine)...)

	serverSystems = append(serverSystems, []ecs.System{
		ecs.System{"ServerSendUpdate", func(dt time.Duration) {
			ServerSendUpdate(engine, sock)
		}},
	}...)

	return serverSystems
}

func CreatePollNetworkSystem(engine *ecs.Engine, networkChannel chan serdes.WorldUpdate) ecs.System {
	sys := ecs.System{"PollNetworkChannel", func(dt time.Duration) {

	MainLoop:
		for {
			select {
			case update := <-networkChannel:
				// log.Println(update)
				// ecs.Write(engine, update.Id, update.Component)
				for id, compList := range update.WorldData {
					for i := range compList {
						// fmt.Printf("HERE %T", compList[i])
						// log.Println(compList[i])
						ecs.Write(engine, id, compList[i])
					}
				}
			default:
				break MainLoop
			}
		}
	}}

	return sys
}
