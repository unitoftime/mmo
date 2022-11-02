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
	"github.com/unitoftime/mmo/game"
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

	collider := physics.NewCircleCollider(8)
	collider.Layer = WallLayer
	collider.HitLayer = BodyLayer

	for i := 0; i < 5; i++ {
		posX, posY := tmap.TileToPosition(tile.TilePosition{mapSize/2 + 5, mapSize/2 + i})

		id := world.NewId()
		ecs.Write(world, id,
			ecs.C(game.TileObject{}),
			ecs.C(tile.Collider{1,1}),
			ecs.C(physics.Transform{
				X: float64(posX),
				Y: float64(posY),
			}),
			ecs.C(collider),
			ecs.C(physics.NewColliderCache()),
		)
	}

	tmap.RecalculateEntities(world)

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

func CreateServerSystems(world *ecs.World, server *Server, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList, tilemap *tile.Tilemap) []ecs.System {
	serverSystems := []ecs.System{
		CreatePollNetworkSystem(world, networkChannel),
	}

	// serverSystems = append(serverSystems,
	// 	CreatePhysicsSystems(world)...)
	serverSystems = append(serverSystems,
		ecs.System{"MoveCharacters", func(dt time.Duration) {
			ecs.Map3(world, func(id ecs.Id, input *physics.Input, transform *physics.Transform, collider *physics.CircleCollider) {
				MoveCharacter(input, transform, collider, tilemap, dt)
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

func MoveCharacter(input *physics.Input, transform *physics.Transform, collider *physics.CircleCollider, tilemap *tile.Tilemap, dt time.Duration) {
	// Note: 100 good starting point, 200 seemed like a good max
	speed := 125.0

	tile, ok := tilemap.Get(tilemap.PositionToTile(float32(transform.X), float32(transform.Y)))
	if ok {
		if tile.Type == WaterTile {
			// Slow the player down if they're on water tile
			speed = speed / 2.0
		}
	}

	// oldTransform := *transform

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

	// newTile, ok := tilemap.Get(tilemap.PositionToTile(float32(transform.X), float32(transform.Y)))
	// if !ok {
	// 	// If !ok then we've gone off the tilemap, just snap back to where we came from
	// 	*transform = oldTransform
	// }
	// if newTile.Entity != ecs.InvalidEntity {
	// 	*transform = oldTransform
	// }


	tilePos := tilemap.GetOverlappingTiles(transform.X, transform.Y, collider)
	for i := range tilePos {
		tile, ok := tilemap.Get(tilePos[i])

		// If no tile exists there or there is any entity positioned on this tile,
		// then just assume its collidable
		if !ok || tile.Entity != ecs.InvalidEntity {
			// *transform = oldTransform
			// minX, minY, maxX, maxY := tilemap.BoundsAt(tilePos[i])
			posX, posY := tilemap.TileToPosition(tilePos[i])

			// resolveW := collider.Radius + float64(tilemap.TileSize[0]/2)
			// resolveH := collider.Radius + float64(tilemap.TileSize[1]/2)

			dx := transform.X - float64(posX)
			dy := transform.Y - float64(posY)

			// clamp
			if dx > float64(tilemap.TileSize[0])/2 {
				dx = float64(tilemap.TileSize[0])/2
			} else if dx < -float64(tilemap.TileSize[0])/2 {
				dx = -float64(tilemap.TileSize[0])/2
			}
			if dy > float64(tilemap.TileSize[1])/2 {
				dy = float64(tilemap.TileSize[1])/2
			} else if dy < -float64(tilemap.TileSize[1])/2 {
				dy = -float64(tilemap.TileSize[1])/2
			}

			// Closest point
			point := physics.V2(dx + float64(posX), dy + float64(posY))
			center := physics.V2(transform.X, transform.Y)

			dv := point.Sub(center)
			response := dv.Norm().Scaled(dv.Len() - collider.Radius)

			// Resolve
			newCenter := center.Add(response)
			if math.Abs(response.X) > math.Abs(response.Y) {
				transform.X = newCenter.X
			} else {
				transform.Y = newCenter.Y
			}
		}
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

	// // Resolve Collisions
	// ecs.Map2(world, func(id ecs.Id, transform *physics.Transform, collider *physics.CircleCollider, cache *physics.ColliderCache) {
	// 	for _, targetId := range cache.Current {
	// 		targetCollider := ecs.Read[physics.CircleCollider](world, targetId)
	// 	}
	// })
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

