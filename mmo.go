package mmo

import (
	// "fmt"
	"time"
	"math"
	"regexp"

	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/tile"
	"github.com/unitoftime/flow/phy2"
	"github.com/unitoftime/flow/pgen"
)

type Input struct {
	Up, Down, Left, Right bool
}

var seed int64 = 12345
var mapSize int = 100
var tileSize int = 16

const (
	NoLayer phy2.CollisionLayer = 0
	BodyLayer phy2.CollisionLayer = 1 << iota
	WallLayer
)

func SpawnPoint() phy2.Pos {
	spawnPoint := phy2.Pos{
		X: float64(tileSize*mapSize/2),
		Y: float64(tileSize*mapSize/2),
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
	collider := phy2.NewCircleCollider(8)
	collider.Layer = WallLayer
	collider.HitLayer = BodyLayer

	ecs.Write(world, id,
		ecs.C(TileObject{}),
		ecs.C(tile.Collider{1,1}),
		ecs.C(phy2.Pos{
			X: math.Round(float64(posX)),
			Y: math.Round(float64(posY)),
		}),
		ecs.C(collider),
		ecs.C(phy2.NewColliderCache()),
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

func MoveCharacter(input *Input, transform *phy2.Pos, collider *phy2.CircleCollider, tilemap *tile.Tilemap, dt time.Duration) {
	// Note: 100 good starting point, 200 seemed like a good max
	speed := 125 * dt.Seconds()

	tile, ok := tilemap.Get(tilemap.PositionToTile(float32(transform.X), float32(transform.Y)))
	if ok {
		if tile.Type == WaterTile {
			// Slow the player down if they're on water tile
			speed = speed / 2.0
		}
	}

	if input.Left {
		transform.X -= speed
	}
	if input.Right {
		transform.X += speed
	}
	if input.Up {
		transform.Y += speed
	}
	if input.Down {
		transform.Y -= speed
	}

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
			point := phy2.V2(dx + float64(posX), dy + float64(posY))
			center := phy2.V2(transform.X, transform.Y)

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
	ecs.Map2(world, func(idA ecs.Id, colA *phy2.CircleCollider, cacheA *phy2.ColliderCache) {
		cacheA.Clear()
		ecs.Map2(world, func(idB ecs.Id, colB *phy2.CircleCollider, cacheB *phy2.ColliderCache) {
			if idA == idB { return } // Skip if collider is the same entity

			if !colA.LayerMask(colB.Layer) { return } // Skip if layer mask doesn't match

			// Check if there is a collision
			if colA.Collides(1.0, colB) {
				cacheA.Add(idB)
			}
		})
	})

	// // Resolve Collisions
	// ecs.Map2(world, func(id ecs.Id, transform *phy2.Transform, collider *phy2.CircleCollider, cache *phy2.ColliderCache) {
	// 	for _, targetId := range cache.Current {
	// 		targetCollider := ecs.Read[phy2.CircleCollider](world, targetId)
	// 	}
	// })
}

const FixedTimeStep time.Duration =  16 * time.Millisecond
func GetScheduler() *ecs.Scheduler {
	schedule := ecs.NewScheduler()
	schedule.SetFixedTimeStep(FixedTimeStep)
	return schedule
}

type TileObject struct {
}

const NumBodyTypes = 4
type Body struct {
	Type uint32
}

type Speech struct {
	Text string
	handledSent, handledRender bool
}

// handles the speech, returns true if the speech wasn't already handled
func (s *Speech) HandleSent() bool {
	if s.handledSent {
		return false
	}

	s.handledSent = true
	return true
}

func (s *Speech) HandleRender() bool {
	if s.handledRender {
		return false
	}

	s.handledRender = true
	return true
}


// This should probably be somewhere else
func FilterChat(msg string) string {
	match, err := regexp.MatchString(`^[\w!@#$%^&*()[{\]}'";:<>,.\/\?~\-_,.+=\\ ]+$`, msg)
	if err != nil {
		log.Error().Err(err).Msg("Regex Matching error")
		return "[This message was delete by moderator.]"
	}
	if match {
		return msg
	} else {
		return "[This message was delete by moderator.]"
	}
}


