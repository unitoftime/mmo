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

// TODO - make this guy generic
type RingBuffer struct {
	idx int
	buffer []time.Duration
}
func NewRingBuffer(length int) *RingBuffer {
	return &RingBuffer{
		idx: 0,
		buffer: make([]time.Duration, length),
	}
}

func (b *RingBuffer) Add(t time.Duration) {
	b.buffer[b.idx] = t
	b.idx = (b.idx + 1) % len(b.buffer)
}

// TODO - Maybe convert this to an iterator
func (b *RingBuffer) Buffer() []time.Duration {
	ret := make([]time.Duration, len(b.buffer))
	firstSliceLen := len(b.buffer) - b.idx
	copy(ret[:firstSliceLen], b.buffer[b.idx:len(b.buffer)])
	copy(ret[firstSliceLen:], b.buffer[0:b.idx])
	return ret
}

// ---

var seed int64 = 12345
var mapSize int = 100
var tileSize int = 16

const (
	NoLayer physics.CollisionLayer = 0
	BodyLayer physics.CollisionLayer = 1 << iota
	WallLayer
)

type InputBufferItem struct {
	Input physics.Input
	Time time.Time
}

// This represents global player data on the client
type PlayerData struct {
	mu sync.RWMutex
	id ecs.Id
	playerTick uint16
	serverTick uint16
	lastMessage string
	inputBuffer []InputBufferItem
	roundTripTimes *RingBuffer
}

func NewPlayerData() *PlayerData {
	return &PlayerData{
		id: ecs.InvalidEntity,
		inputBuffer: make([]InputBufferItem, 0),
		roundTripTimes: NewRingBuffer(100), // TODO - configurable
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
	// fmt.Println("SetTicks: ", serverTick, serverUpdatePlayerTick, time.Now())

	p.mu.Lock()
	defer p.mu.Unlock()

	// Set the last server tick we've received
	p.serverTick = serverTick

	// Cut off every player input tick that the server hasn't processed
	cut := int(p.playerTick - serverUpdatePlayerTick)
	// fmt.Println("InputBuffer", p.serverTick, p.playerTick, serverUpdatePlayerTick, len(p.inputBuffer))
	for i := 0; i < len(p.inputBuffer)-cut; i++ {
		p.roundTripTimes.Add(time.Since(p.inputBuffer[i].Time))
	}

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
	p.inputBuffer = append(p.inputBuffer, InputBufferItem{
		Input: input,
		Time: time.Now(),
	})
	return p.playerTick
}

func (p *PlayerData) GetInputBuffer() []InputBufferItem {
	return p.inputBuffer
}

func (p *PlayerData) RoundTripTimes() []time.Duration {
	return p.roundTripTimes.Buffer()
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

	walls := []tile.TilePosition{
		// North wall
		tile.TilePosition{mapSize/2 + 5, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 + 4, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 + 3, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 + 2, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 + 1, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 + 0, mapSize/2 + 5},

		tile.TilePosition{mapSize/2 - 5, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 - 4, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 - 3, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 - 2, mapSize/2 + 5},
		tile.TilePosition{mapSize/2 - 1, mapSize/2 + 5},

		// South wall(ish)
		tile.TilePosition{mapSize/2 + 5, mapSize/2 - 5},
		tile.TilePosition{mapSize/2 + 4, mapSize/2 - 5},
		tile.TilePosition{mapSize/2 + 3, mapSize/2 - 5},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 - 5},
		tile.TilePosition{mapSize/2 - 4, mapSize/2 - 5},
		tile.TilePosition{mapSize/2 - 3, mapSize/2 - 5},

		// East Wall
		tile.TilePosition{mapSize/2 + 5, mapSize/2 + 4},
		tile.TilePosition{mapSize/2 + 5, mapSize/2 + 3},
		tile.TilePosition{mapSize/2 + 5, mapSize/2 + 2},
		tile.TilePosition{mapSize/2 + 5, mapSize/2 + 1},
		tile.TilePosition{mapSize/2 + 5, mapSize/2 + 0},

		tile.TilePosition{mapSize/2 + 5, mapSize/2 - 1},
		tile.TilePosition{mapSize/2 + 5, mapSize/2 - 2},
		tile.TilePosition{mapSize/2 + 5, mapSize/2 - 3},
		tile.TilePosition{mapSize/2 + 5, mapSize/2 - 4},
		tile.TilePosition{mapSize/2 + 5, mapSize/2 - 5},

		// West Wall
		tile.TilePosition{mapSize/2 - 5, mapSize/2 + 4},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 + 3},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 + 2},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 + 1},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 + 0},

		tile.TilePosition{mapSize/2 - 5, mapSize/2 - 1},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 - 2},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 - 3},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 - 4},
		tile.TilePosition{mapSize/2 - 5, mapSize/2 - 5},
	}
	for _, pos := range walls {
		addWall(world, tmap, pos)
	}

	tmap.RecalculateEntities(world)

	return tmap
}

func addWall(world *ecs.World, tilemap *tile.Tilemap, pos tile.TilePosition) {
	posX, posY := tilemap.TileToPosition(pos)

	id := world.NewId()

	// TODO - make square collider
	collider := physics.NewCircleCollider(8)
	collider.Layer = WallLayer
	collider.HitLayer = BodyLayer

	// log.Print(math.Round(float64(posX)), math.Round(float64(posY)))

	ecs.Write(world, id,
		ecs.C(game.TileObject{}),
		ecs.C(tile.Collider{1,1}),
		ecs.C(physics.Transform{
			X: math.Round(float64(posX)),
			Y: math.Round(float64(posY)),
		}),
		ecs.C(collider),
		ecs.C(physics.NewColliderCache()),
	)
}

const (
	GrassTile tile.TileType = iota
	DirtTile
	WaterTile
	ConcreteTile
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

			mid := mapSize/2
			if x <= mid+5 && x >= mid-5 && y <= mid+5 && y >= mid-5 {
				tiles[x][y] = tile.Tile{ConcreteTile, 0, ecs.InvalidEntity}
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

			// Check if they are even still colliding (this fact may change after one tile gets resolved)
			// TODO - this should cleanup, I need some rect and circle primitives to do these checks with. Basically if the distance in the X and the Y are both larger than the circle radius plus the tileSize/2. Then the circle is already outside the bounds
			// log.Print("Math ", math.Abs(dx), collider.Radius + float64(tilemap.TileSize[0]/2), math.Abs(dy), collider.Radius + float64(tilemap.TileSize[1]/2))
			// TODO - Should I add any thresholding here? I think most of the time the floats are like exactly the same
			if math.Abs(dx) >= collider.Radius + float64(tilemap.TileSize[0]/2) || math.Abs(dy) >= collider.Radius + float64(tilemap.TileSize[1]/2) {
				continue // Skip if the circle is no longer overlapping this tile
			}

			// clamp
			if dx > float64(tilemap.TileSize[0]/2) {
				dx = float64(tilemap.TileSize[0]/2)
			} else if dx < -float64(tilemap.TileSize[0]/2) {
				dx = -float64(tilemap.TileSize[0]/2)
			}
			if dy > float64(tilemap.TileSize[1]/2) {
				dy = float64(tilemap.TileSize[1]/2)
			} else if dy < -float64(tilemap.TileSize[1]/2) {
				dy = -float64(tilemap.TileSize[1]/2)
			}

			// Closest point
			point := physics.V2(dx + float64(posX), dy + float64(posY))
			center := physics.V2(transform.X, transform.Y)

			dv := point.Sub(center)
			response := dv.Norm().Scaled(dv.Len() - collider.Radius)

			// Resolve
			newCenter := center.Add(response)
			if math.Abs(response.X) >= math.Abs(response.Y) {
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

// type EcsUpdate struct {
// 	WorldData map[ecs.Id][]ecs.Component
// 	Delete []ecs.Id
// }

// TODO - this kindof represents a greater pattern of trying to apply commands to the world in a threadsafe manner. Maybe integrate this into the ECS library: https://docs.rs/bevy/0.4.0/bevy/ecs/trait.Command.html
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

