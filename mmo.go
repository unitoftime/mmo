package mmo

import (
	// "fmt"
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

const (
	NoLayer physics.CollisionLayer = 0
	BodyLayer physics.CollisionLayer = 1 << iota
	WallLayer
)

// This represents global player data on the client
type PlayerData struct {
	mu sync.RWMutex
	id ecs.Id
	playerTick uint16
	serverTick uint16
	lastMessage string
	inputBuffer []physics.Input
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

// func (p *PlayerData) Tick() uint16 {
// 	p.mu.RLock()
// 	ret := p.tick
// 	p.mu.RUnlock()
// 	return ret
// }

func (p *PlayerData) SetTicks(serverTick, serverUpdatePlayerTick uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Set the last server tick we've received
	p.serverTick = serverTick

	// Cut off every player input tick that the server hasn't processed
	cut := int(p.playerTick - serverUpdatePlayerTick)
	// fmt.Println("InputBuffer", p.serverTick, p.playerTick, serverUpdatePlayerTick, len(p.inputBuffer))
	if cut >= 0 && cut <= len(p.inputBuffer) {
		// TODO - it'd be more efficient to use a queue
		copy(p.inputBuffer, p.inputBuffer[len(p.inputBuffer)-cut:])
		p.inputBuffer = p.inputBuffer[:cut]
		// fmt.Println("Copied", n, len(p.inputBuffer))
	} else {
		// fmt.Println("OOB")
	}
}

// Returns the player tick that this input is associated with
func (p *PlayerData) AppendInputTick(input physics.Input) uint16 {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.playerTick = (p.playerTick + 1) % math.MaxUint16
	p.inputBuffer = append(p.inputBuffer, input)
	return p.playerTick
}

func (p *PlayerData) GetInputBuffer() []physics.Input {
	return p.inputBuffer
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

// This is the tick that the client says they are on
type ClientTick struct {
	Tick uint16 // This is the tick that the player is currently on
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

// Note: This didn't work because client has to handle input differently than server (ie client does clientside prediction and interp)
// func CreatePhysicsSystems(world *ecs.World) []ecs.System {
// 	physicsSystems := []ecs.System{
// 		ecs.System{"HandleInput", func(dt time.Duration) {
// 			physics.HandleInput(world, dt)
// 		}},
// 	}
// 	return physicsSystems
// }

func CreateServerSystems(world *ecs.World, server *Server, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList) []ecs.System {
	serverSystems := []ecs.System{
		CreatePollNetworkSystem(world, networkChannel),
	}

	// serverSystems = append(serverSystems,
	// 	CreatePhysicsSystems(world)...)
	serverSystems = append(serverSystems,
		ecs.System{"MoveCharacters", func(dt time.Duration) {
			ecs.Map2(world, func(id ecs.Id, input *physics.Input, transform *physics.Transform) {
				MoveCharacter(input, transform, dt)
			})
		}},
		ecs.System{"CheckCollisions", func(dt time.Duration) {
			// Set the collider position
			ecs.Map2(world, func(id ecs.Id, transform *physics.Transform, col *physics.CircleCollider) {
				col.CenterX = transform.X
				col.CenterY = transform.Y
			})

			CheckCollisions(world)
		}},
	)

	serverSystems = append(serverSystems, []ecs.System{
		ecs.System{"ServerSendUpdate", func(dt time.Duration) {
			ServerSendUpdate(world, server, deleteList)
		}},
	}...)

	return serverSystems
}

func MoveCharacter(input *physics.Input, transform *physics.Transform, dt time.Duration) {
	// Note: 100 good starting point, 200 seemed like a good max
	speed := 125.0

	if input.Left {
		transform.X -= speed * dt.Seconds()
	}
	if input.Right {
		transform.X += speed * dt.Seconds()
	}
	if input.Up {
		transform.Y += speed * dt.Seconds()
	}
	if input.Down {
		transform.Y -= speed * dt.Seconds()
	}
}

func CheckCollisions(world *ecs.World) {
	// Detect all collisions
	ecs.Map2(world, func(idA ecs.Id, colA *physics.CircleCollider, cacheA *physics.ColliderCache) {
		cacheA.Clear()
		ecs.Map2(world, func(idB ecs.Id, colB *physics.CircleCollider, cacheB *physics.ColliderCache) {
			if idA == idB { return } // Skip if collider is the same entity

			if !colA.LayerMask(colB.Layer) { return } // Skip if layer mask doesn't match

			// Check if there is a collision
			if colA.Collides(1.0, colB) {
				cacheA.Add(idB)
			}
		})
	})
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

