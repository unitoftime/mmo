package client

import (
	"time"
	// "math"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/glitch"

	"github.com/unitoftime/flow/ds"
	"github.com/unitoftime/flow/phy2"
	// "github.com/unitoftime/flow/render"
	// "github.com/unitoftime/flow/interp"
	"github.com/unitoftime/flow/tile"
	"github.com/unitoftime/flow/net"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

// This is mostly for debug, but maybe its a good thing to track
type ServerTransform struct {
	phy2.Pos
	Handled bool
	ServerTick uint16
	PlayerTick uint16
}

type NextTransform struct {
	// PhyTrans phy2.Transform
	ReadIdx, WriteIdx int
	Transforms []ServerTransform
	Replayed bool
	PreExtInterpTo ServerTransform // The position to interp to before we add extrapolation
	InterpTo ServerTransform // The current position to interp to
	InterpFrom ServerTransform // The current position to interp from
	Interp float64

	Extrapolation phy2.Pos // This is the extrapolated position for the currently interpolated tick
	ExtrapolationOffset phy2.Pos

	Remaining, Total time.Duration
	AvgTickTime time.Duration
	TargetDelay int // Delay in ticks

	LastTick uint16
}
func NewTransformBuffer() NextTransform {
	return NextTransform{
		ReadIdx: 0,
		WriteIdx: 0,
		Transforms: make([]ServerTransform, 12),
		Replayed: false,
		Interp: 0,
		AvgTickTime: 16 * time.Millisecond * 4, // TODO - get from actual average of ticks coming in. Note: 4 comes from %4 that I'm doing on sending ticks
		TargetDelay: -5, // TODO - Set
	}
}

func (n *NextTransform) Len() int {
	l := len(n.Transforms)

	firstIdx := n.ReadIdx
	lastIdx := n.WriteIdx
	if lastIdx < firstIdx {
		lastIdx += l
	}
	return lastIdx - firstIdx
}

func (n *NextTransform) Delay() int {
	if n.Len() == 0 {
		return 0
	}
	firstIdx := n.ReadIdx
	lastIdx := (n.WriteIdx + len(n.Transforms) - 1) % len(n.Transforms)

	first := n.Transforms[firstIdx]
	last := n.Transforms[lastIdx]

	// log.Print("Delay(): ", firstIdx, lastIdx, first.ServerTick, last.ServerTick)
	// for i := range n.Transforms {
	// 	log.Print(n.Transforms[i])
	// }
	return int(last.ServerTick - first.ServerTick)
}

func (n *NextTransform) Add(t ServerTransform) {
	if (n.WriteIdx + 1) % len(n.Transforms) == n.ReadIdx {
		// Queue is full
		panic("QUEUE IS FULL!") // TODO - Not sure how to handle this, maybe just keep writing like I would in a ringbuffer?
	}
	n.Transforms[n.WriteIdx] = t
	n.WriteIdx = (n.WriteIdx + 1) % len(n.Transforms)
}

func (n *NextTransform) Peek() (ServerTransform, bool) {
	if n.ReadIdx == n.WriteIdx {
		return ServerTransform{}, false
	}
	return n.Transforms[n.ReadIdx], true
}
func (n *NextTransform) PeekLast() (ServerTransform, bool) {
	if n.ReadIdx == n.WriteIdx {
		return ServerTransform{}, false
	}
	idx := (n.WriteIdx + len(n.Transforms) - 1) % len(n.Transforms)
	return n.Transforms[idx], true
}
func (n *NextTransform) Remove() (ServerTransform, bool) {
	if n.ReadIdx == n.WriteIdx {
		return ServerTransform{}, false
	}
	val := n.Transforms[n.ReadIdx]
	n.ReadIdx = (n.ReadIdx + 1) % len(n.Transforms)
	return val, true
}

// func (n *NextTransform) GetBack(i int) *phy2.Transform {
// 	l := len(n.Transforms)
// 	idx := (n.ReadIdx + l - 1 - i) % l
// 	return &n.Transforms[idx]
// }

// // Returns the last one added (the newest element)
// func (n *NextTransform) Last() *phy2.Transform {
// 	l := len(n.Transforms)
// 	return &n.Transforms[(n.ReadIdx + l - 1) % l]
// }

// TODO - should this operate on pointers?
func (n *NextTransform) Map(fn func(t ServerTransform)) {
	if n.ReadIdx == n.WriteIdx {
		return // Empty
	}

	l := len(n.Transforms)
	firstIdx := n.ReadIdx
	// lastIdx := n.WriteIdx
	lastIdx := (n.WriteIdx + len(n.Transforms) - 1) % len(n.Transforms)

	cnt := 0
	// TODO - this might be simpler in two loops?
	for i := firstIdx; i != lastIdx; i=(i + 1) % l {
		fn(n.Transforms[i])
		cnt++
	}
	// log.Print("Mapped: ", cnt)
}

// type NextTransform struct {
// 	// PhyTrans phy2.Transform
// 	Idx int
// 	Transforms []phy2.Transform
// 	Replayed bool
// 	InterpStart phy2.Transform
// 	Interp float64
// }
// func NewTransformBuffer() NextTransform {
// 	return NextTransform{
// 		Idx: 0,
// 		Transforms: make([]phy2.Transform, 10),
// 		Replayed: false,
// 		Interp: 0,
// 	}
// }

// func (n *NextTransform) Add(t phy2.Transform) {
// 	n.Transforms[n.Idx] = t
// 	n.Idx = (n.Idx + 1) % len(n.Transforms)
// }

// // Returns the one after the last one added (the oldest element of the buffer, or the first element of the buffer)
// func (n *NextTransform) First() *phy2.Transform {
// 	return &n.Transforms[n.Idx]
// }

// func (n *NextTransform) GetBack(i int) *phy2.Transform {
// 	l := len(n.Transforms)
// 	idx := (n.Idx + l - 1 - i) % l
// 	return &n.Transforms[idx]
// }

// // Returns the last one added (the newest element)
// func (n *NextTransform) Last() *phy2.Transform {
// 	l := len(n.Transforms)
// 	return &n.Transforms[(n.Idx + l - 1) % l]
// }

// func (n *NextTransform) Map(fn func(t phy2.Transform)) {
// 	l := len(n.Transforms)
// 	firstIdx := n.Idx
// 	lastIdx := (n.Idx + l - 1) % l

// 	// TODO - this might be simpler in two loops?
// 	for i := firstIdx; i != lastIdx; i=(i + 1) % l {
// 		fn(n.Transforms[i])
// 	}
// }

// type LastTransform phy2.Transform
// TODO - do a full fledged transform buffer
// type TransformBuffer struct {
// 	buffer []phy2.Transform
// }

func CreateClientSystems(world *ecs.World, sock *net.Socket, playerData *PlayerData, tilemap *tile.Tilemap) []ecs.System {
	clientSystems := []ecs.System{
		ecs.System{"ClientSendUpdate", func(dt time.Duration) {
			ClientSendUpdate(world, sock, playerData)
		}},
		// ecs.System{"ReplayInputBuffer", func(dt time.Duration) {
		// 	playerId := playerData.Id()

		// 	// Set ServerTransforms to NextTransforms
		// 	serverTransform, ok := ecs.Read[ServerTransform](world, playerId)
		// 	if !ok { return } // Skip if player doesn't have a transform

		// 	// playerData.SetTicks(serverTransform.ServerTick, serverTransform.PlayerTick)

		// 	transformBuffer, ok := ecs.Read[NextTransform](world, playerId)
		// 	if !ok {
		// 		// Create TransformBuffer if it doesn't exist
		// 		transformBuffer = NewTransformBuffer()
		// 	}
		// 	if !serverTransform.Handled {
		// 		serverTransform.Handled = true
		// 		ecs.Write(world, playerId, ecs.C(serverTransform))

		// 		curTransform, ok := ecs.Read[phy2.Transform](world, playerId)
		// 		if !ok {
		// 			// Create TransformBuffer if it doesn't exist
		// 			curTransform = phy2.Transform{}
		// 			ecs.Write(world, playerId, ecs.C(curTransform))
		// 		}

		// 		// transform.InterpStart = curTransform

		// 		// Replays the remaining input buffer to try and guesstimate the player's position
		// 		inputBuffer := playerData.GetInputBuffer()
		// 		// log.Print("InputBuffer: ", len(inputBuffer))

		// 		// transform, ok := ecs.Read[NextTransform](world, playerId)
		// 		// if !ok { return } // Skip if player doesn't have a transform
		// 		collider, ok := ecs.Read[phy2.CircleCollider](world, playerId)
		// 		if !ok { return } // Skip if player doesn't have a collider

		// 		predictedTransform := serverTransform
		// 		for i := range inputBuffer {
		// 			for ii := 0; ii < 4; ii++ { // TODO - 4 because we do %4 on send rate
		// 				mmo.MoveCharacter(&inputBuffer[i].Input, &(predictedTransform.Transform), &collider, tilemap, mmo.FixedTimeStep)
		// 			}
		// 		}

		// 		transformBuffer.Add(predictedTransform)
		// 		ecs.Write(world, playerId, ecs.C(transformBuffer))
		// 	}
		// }},
		// Note: This must be after ReplayInputBuffer because if a replay happens, then we want to interpolate halfway there
		ecs.System{"InterpolateSpritePositions", func(dt time.Duration) {
			// TODO - hack. We needed a way to create the transform component for other players (because we did a change which makes us set NextTransform over the wire instead of transform. So those were never being set
			// Logic: If has next transform, but doesn't have transform, then add transform
			// ecs.Map(world, func(id ecs.Id, nextT *NextTransform) {
			// 	_, ok := ecs.Read[phy2.Transform](world, id)
			// 	if !ok {
			// 		ecs.Write(world, id, ecs.C(phy2.Transform{}))
			// 	}
			// })

			playerId := playerData.Id()
			ecs.Map(world, func(id ecs.Id, serverTransform *ServerTransform) {
				pos, ok := ecs.Read[phy2.Pos](world, id)
				if !ok {
					pos = phy2.Pos{}
					ecs.Write(world, id, ecs.C(pos))
				}

				transformBuffer, ok := ecs.Read[NextTransform](world, id)
				if !ok {
					transformBuffer = NewTransformBuffer()
				}

				if !serverTransform.Handled {
					// log.Print("New ServerTransform")
					serverTransform.Handled = true
					ecs.Write(world, id, ecs.C(serverTransform))

					// transformBuffer.Add(*serverTransform)

					transformBuffer.InterpFrom = transformBuffer.InterpTo
					transformBuffer.InterpFrom.Pos = pos
					transformBuffer.InterpTo = *serverTransform

					transformBuffer.Total = 64 * time.Millisecond //TODO! - Hardcoding 4x fixed time step
					// Note: Add like a single frame of time to the total so that we get there a little bit slower. Ideally this would smooth the transitions between frames b/c we'd always lag behind.
					transformBuffer.Total = transformBuffer.Total + 32 * time.Millisecond

					transformBuffer.Remaining = transformBuffer.Total

					// Extrapolate with trimmed player input buffer
					extrapolatedPos := transformBuffer.InterpTo.Pos
					if id == playerId {

						inputBuffer := playerData.GetInputBuffer()
						// log.Print("InputBufLen: ", len(inputBuffer))
						collider, ok := ecs.Read[phy2.CircleCollider](world, playerId)
						if !ok { return } // Skip if player doesn't have a collider

						for i := range inputBuffer {
							// for ii := 0; ii < 4; ii++ { // TODO - 4 because we do %4 on send rate
								// mmo.MoveCharacter(&inputBuffer[i].Input, &dest.Transform, &collider, tilemap, mmo.FixedTimeStep)
								mmo.MoveCharacter(&inputBuffer[i].Input, &extrapolatedPos, &collider, tilemap, mmo.FixedTimeStep)
							// }
						}
						transformBuffer.Extrapolation = extrapolatedPos
						// transformBuffer.ExtrapolationOffset.X = transformBuffer.ExtrapolationOffset.X

						// TODO! - this looks bad when there is high packet loss. This is caused by multiple inputs that the client sends getting compressed into a single tick.
						// transformBuffer.InterpTo.Transform.X += extrapolatedPos.X
						// transformBuffer.InterpTo.Transform.Y += extrapolatedPos.X
						transformBuffer.PreExtInterpTo = transformBuffer.InterpTo
						// transformBuffer.InterpTo.Transform.X = extrapolatedPos.X
						// transformBuffer.InterpTo.Transform.Y = extrapolatedPos.Y
						// log.Print("Extrap: ", extrapolatedPos, transformBuffer.InterpFrom.Transform, transformBuffer.InterpTo.Transform)
					}
				}

				ecs.Write(world, id, ecs.C(transformBuffer))
			})

			// NOTE: I moved this to render tick to smooth it out a bit more
			// playerPhyT, _ := ecs.Read[phy2.Transform](world, playerId)
			// ecs.Map2(world, func(id ecs.Id, phyT *phy2.Transform, nextT *NextTransform) {
			// 	nextT.Remaining -= dt

			// 	interpFactor := 1 - (nextT.Remaining.Seconds() / nextT.Total.Seconds())
			// 	if interpFactor > 1 { // TODO - can I prevent this from going above, makes it stop for a second/frame
			// 		interpFactor = 1
			// 	}

			// 	old := *phyT
			// 	phyT.X = interp.Linear.Float64(nextT.InterpFrom.X, nextT.InterpTo.X, interpFactor)
			// 	phyT.Y = interp.Linear.Float64(nextT.InterpFrom.Y, nextT.InterpTo.Y, interpFactor)

			// 	log.Print("Trans: ", interpFactor, phyT.Sub(&playerPhyT), phyT.Sub(&old))
			// })





			
			// // TODO - Note: Keybinds is only included so that clients don't simulate another client's input, I should isolate these another way
			// ecs.Map4(world, func(id ecs.Id, input *phy2.Input, keybinds *render.Keybinds, nextTrans *NextTransform, collider *phy2.CircleCollider) {
			// 	// Extrapolate with trimmed player input buffer
			// 	if id != playerId { return }

			// 	phyT, ok := ecs.Read[phy2.Transform](world, id)
			// 	if !ok {
			// 		panic("AAAA")
			// 	}
			// 	old := phyT

			// 	// if !nextTrans.Replayed {
			// 	// 	nextTrans.Replayed = true
			// 	inputBuffer := playerData.GetInputBuffer()
			// 	log.Print("InputBufLen: ", len(inputBuffer))

			// 	// extrapolatedPos := phyT
			// 	for i := range inputBuffer {
			// 		for ii := 0; ii < 4; ii++ { // TODO - 4 because we do %4 on send rate
			// 			// mmo.MoveCharacter(&inputBuffer[i].Input, &dest.Transform, &collider, tilemap, mmo.FixedTimeStep)
			// 			mmo.MoveCharacter(&inputBuffer[i].Input, &phyT, collider, tilemap, mmo.FixedTimeStep)
			// 		}
			// 	}
			// 	mmo.MoveCharacter(input, &phyT, collider, tilemap, dt)
			// 	log.Print("Extra: ", phyT.Sub(&playerPhyT), phyT.Sub(&old))
			// 	ecs.Write(world, id, ecs.C(phyT))

			// 	// nextTrans.Extrapolation = extrapolatedPos
			// 	// // nextTrans.ExtrapolationOffset.X = nextTrans..X
			// 	// // nextTrans.ExtrapolationOffset.X = nextTrans.ExtrapolationOffset.X

			// 	// // TODO! - this looks bad when there is high packet loss. This is caused by multiple inputs that the client sends getting compressed into a single tick.
			// 	// nextTrans.InterpTo.Transform.X += nextTrans.ExtrapolationOffset.X
			// 	// nextTrans.InterpTo.Transform.Y += nextTrans.ExtrapolationOffset.Y
			// 	// // log.Print("Extrap: ", extrapolatedPos, nextTrans.InterpFrom.Transform, nextTrans.InterpTo.Transform)
			// 	// // }

			// 	// mmo.MoveCharacter(input, &nextTrans.InterpFrom.Transform, collider, tilemap, dt)
			// })


/*
			ecs.Map(world, func(id ecs.Id, serverTransform *ServerTransform) {
				_, ok := ecs.Read[phy2.Transform](world, id)
				if !ok {
					ecs.Write(world, id, ecs.C(phy2.Transform{}))
				}
				transformBuffer, ok := ecs.Read[NextTransform](world, id)
				if !ok {
					transformBuffer = NewTransformBuffer()
				}

				if !serverTransform.Handled {
					serverTransform.Handled = true
					ecs.Write(world, id, ecs.C(serverTransform))

					transformBuffer.Add(*serverTransform)

					// log.Print("GOTNEWNEWORKTICK")
				}

				ecs.Write(world, id, ecs.C(transformBuffer))
			})

			// This interpolates the transform position based on what the server just said it was
			const maxInterp float64 = 6 * 16.0
			const minInterp float64 = 2.0
			playerId := playerData.Id()
			ecs.Map2(world, func(id ecs.Id, phyT *phy2.Transform, nextT *NextTransform) {

				nextT.Remaining -= dt
				// log.Print("Remaining: ", nextT.Remaining)
				if nextT.Remaining <= 0 {
					// log.Print("PULL NETWORK TICK")
					dest, ok := nextT.Remove()

					if ok {
						start := nextT.InterpTo.Transform
						// start := *phyT

						// Replay interp
						extrapolatedPos := phy2.Transform{}
						if id == playerId {
							playerData.SetTicks(nextT.InterpTo.ServerTick, nextT.InterpTo.PlayerTick)

							inputBuffer := playerData.GetInputBuffer()
							log.Print("InputBufLen: ", len(inputBuffer))
							collider, ok := ecs.Read[phy2.CircleCollider](world, playerId)
							if !ok { return } // Skip if player doesn't have a collider

							for i := range inputBuffer {
								for ii := 0; ii < 4; ii++ { // TODO - 4 because we do %4 on send rate
									// mmo.MoveCharacter(&inputBuffer[i].Input, &dest.Transform, &collider, tilemap, mmo.FixedTimeStep)
									mmo.MoveCharacter(&inputBuffer[i].Input, &extrapolatedPos, &collider, tilemap, mmo.FixedTimeStep)
								}
							}
						}

						nextT.InterpFrom = nextT.InterpTo
						nextT.InterpFrom.Transform = start // TODO - does this make sense?
						nextT.InterpTo = dest
						nextT.Extrapolation = extrapolatedPos

						// If deltaTicks is 1, it means that no packets were dropped
						deltaTicks := nextT.InterpTo.ServerTick - nextT.InterpFrom.ServerTick
						if deltaTicks > 10 {
							deltaTicks = 1 // It's just so far off that we are probably going to snap anyways
						} else if deltaTicks < 0 {
							log.Print("NEGATIVE: ", nextT.InterpTo, nextT.InterpFrom)
						}
						// log.Print("DeltaTick: ", deltaTicks, nextT.InterpFrom.ServerTick, nextT.InterpTo.ServerTick)

						// nextT.Total = time.Duration(deltaTicks) * nextT.AvgTickTime

						// delayOffset := nextT.TargetDelay - nextT.Delay()
						// // log.Print("DelayOffset: ", delayOffset, nextT.Len(), nextT.Delay(),  nextT.TargetDelay)
						// slope := 1 // 4 = 16ms / 4
						// timeOffset := delayOffset * slope
						// nextT.Total = nextT.Total + time.Duration(timeOffset) * time.Millisecond
						// // timeOffset := delayOffset * 1
						// // nextT.Total = nextT.Total + time.Duration(timeOffset) * time.Microsecond
						// // nextT.Total = nextT.Total + time.Duration(delayOffset) * time.Millisecond

						// nextT.Remaining = nextT.Total - dt

						nextT.Total = time.Duration(deltaTicks) * nextT.AvgTickTime
						delayOffset := nextT.TargetDelay - nextT.Delay()
						if delayOffset < 0 {
							nextT.Total = nextT.Total - (17 * time.Millisecond)
						}//  else if delayOffset > 0 {
						// 	nextT.Total = nextT.Total + (17 * time.Millisecond)
						// }
						// nextT.Total = nextT.Total + time.Duration(delayOffset) * time.Millisecond
						nextT.Remaining = nextT.Total

						// log.Print("DelayOffset: ", deltaTicks, delayOffset, nextT.Len(), nextT.Delay(),  nextT.TargetDelay, nextT.Total, nextT.Remaining)

						// log.Print("InterpWithTime: ", nextT.Remaining, nextT.Total)
					} else {
						log.Print("NONE TO REMOVE")
					}
				}

				// current := phy2.V2(phyT.X, phyT.Y)
				// next := phy2.V2(nextT.InterpTo.X, nextT.InterpTo.Y)
				// delta := next.Sub(current)

				// Snap, rather than interpolate if the distance is large enough
				// if delta.Len() > maxInterp {
				// 	// phyT.X = nextT.InterpTo.X + nextT.ExtrapolationOffset.X
				// 	// phyT.Y = nextT.InterpTo.Y + nextT.ExtrapolationOffset.Y
				// 	phyT.X = nextT.InterpTo.X
				// 	phyT.Y = nextT.InterpTo.Y
				// 	return
				// }

				// if delta.Len() < minInterp {
				// 	// phyT.X = nextT.InterpTo.X + nextT.ExtrapolationOffset.X
				// 	// phyT.Y = nextT.InterpTo.Y + nextT.ExtrapolationOffset.Y
				// 	// return
				// }

				interpFactor := 1 - (nextT.Remaining.Seconds() / nextT.Total.Seconds())
				if interpFactor <= 1 {
					old := *phyT
					phyT.X = interp.Linear.Float64(nextT.InterpFrom.X, nextT.InterpTo.X, interpFactor)
					phyT.Y = interp.Linear.Float64(nextT.InterpFrom.Y, nextT.InterpTo.Y, interpFactor)

					log.Print("Trans: ", phyT.DistanceTo(&old), nextT.InterpFrom.DistanceTo(&nextT.InterpTo.Transform))

					// // old := *phyT
					// phyT.X = interp.Linear.Float64(nextT.InterpFrom.X, nextT.InterpTo.X, interpFactor)
					// phyT.Y = interp.Linear.Float64(nextT.InterpFrom.Y, nextT.InterpTo.Y, interpFactor)
					// // log.Print("INTERP: ", nextT.InterpStart, nextT.InterpTo, phyT, interpFactor)
					// // log.Print("Trans: ", old, phyT, phyT.DistanceTo(&old), nextT.InterpFrom.DistanceTo(&nextT.InterpTo.Transform))

					// log.Print("Extrap: ", phyT, nextT.Extrapolation)
					// extraFactor := 0.1
					// phyT.X = interp.Linear.Float64(phyT.X, phyT.X + nextT.Extrapolation.X, extraFactor)
					// phyT.Y = interp.Linear.Float64(phyT.Y, phyT.X + nextT.Extrapolation.Y, extraFactor)

					// nextT.Extrapolation.X -= nextT.Extrapolation.X * extraFactor
					// nextT.Extrapolation.Y -= nextT.Extrapolation.Y * extraFactor
				}

				// extraFactor := 0.01
				// nextT.ExtrapolationOffset.X = interp.Linear.Float64(nextT.ExtrapolationOffset.X, nextT.Extrapolation.X, extraFactor)
				// nextT.ExtrapolationOffset.Y = interp.Linear.Float64(nextT.ExtrapolationOffset.Y, nextT.Extrapolation.Y, extraFactor)

				// log.Print("Extrap: ", nextT.Extrapolation, nextT.ExtrapolationOffset)
				// log.Print("InterpX: ", nextT.InterpFrom.X + nextT.ExtrapolationOffset.X, nextT.InterpTo.X + nextT.ExtrapolationOffset.X)
				// log.Print("InterpY: ", nextT.InterpFrom.Y + nextT.ExtrapolationOffset.Y, nextT.InterpTo.Y + nextT.ExtrapolationOffset.Y)
				// phyT.X += nextT.ExtrapolationOffset.X
				// phyT.Y += nextT.ExtrapolationOffset.Y
			})
*/
		}},
	}

	physicsSystems := []ecs.System{
		// ecs.System{"MoveCharacters", func(dt time.Duration) {
		// 	// TODO - Note: Keybinds is only included so that clients don't simulate another client's input, I should isolate these another way
		// 	ecs.Map4(world, func(id ecs.Id, input *phy2.Input, keybinds *render.Keybinds, nextTrans *NextTransform, collider *phy2.CircleCollider) {

		// 		next := nextTrans.First()
		// 		mmo.MoveCharacter(input, next, collider, tilemap, dt)

		// 		// mmo.MoveCharacter(input, &nextTrans.PhyTrans, collider, tilemap, dt)
		// 	})
		// }},
		ecs.System{"SetupColliders", func(dt time.Duration) {
			// Set the collider position
			ecs.Map2(world, func(id ecs.Id, transform *NextTransform, col *phy2.CircleCollider) {
				next := transform.InterpTo
				col.CenterX = next.X
				col.CenterY = next.Y

				// col.CenterX = transform.PhyTrans.X
				// col.CenterY = transform.PhyTrans.Y
			})
		}},
		ecs.System{"CheckCollisions", func(dt time.Duration) {
			mmo.CheckCollisions(world)
		}},
	}
	clientSystems = append(clientSystems, physicsSystems...)
	return clientSystems
}

var everyOther int

func ClientSendUpdate(world *ecs.World, clientConn *net.Socket, playerData *PlayerData) {
	// everyOther = (everyOther + 1) % 4
	// if everyOther != 0 {
	// 	return // skip
	// }

	playerId := playerData.Id()
	// if clientConn is closed for some reason, then we won't be able to send
	// TODO - With the atomic this fast enough?
	connected := clientConn.Connected.Load()
	if !connected { return } // Exit early because we are not connected

	input, ok := ecs.Read[mmo.Input](world, playerId)
	if !ok { return } // If we can't find the players input just exit early

	playerTick := playerData.AppendInputTick(input)

	compSlice := []ecs.Component{
		ecs.C(input),
	}

	// lastMsg := playerData.GetLastMessage()
	// // log.Print(lastMsg)
	// var messages []mmo.ChatMessage
	// if lastMsg != nil {
	// 	messages = []mmo.ChatMessage{
	// 		mmo.ChatMessage{
	// 			Username: "", // Note: Can't trust the username that the client sends
	// 			Message: lastMsg.Message,
	// 		},
	// 	}
	// }

	// If we can't find a speech, that's okay
	speech, speechFound := ecs.Read[mmo.Speech](world, playerId)
	if speechFound {
		if speech.HandleSent() {
			compSlice = append(compSlice, ecs.C(speech))
			ecs.Write(world, playerId, ecs.C(speech))
		}
	}

	// log.Print(messages)

	update := serdes.WorldUpdate{
		PlayerTick: playerTick,
		WorldData: map[ecs.Id][]ecs.Component{
			playerId: compSlice,
		},
		// Messages: messages,
	}
	// log.Print("ClientSendUpdate:", update)

	// log.Print(update)

	// Duplicate Sends to counter packet loss
	for i := 0; i < 4; i++ {
		err := clientConn.Send(update)
		if err != nil {
			log.Warn().Err(err).Msg("ClientSendUpdate")
		}
	}

	// ecs.Map2(world, func(id ecs.Id, _ *ClientOwned, input *phy2.Input) {
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

var AvgWorldUpdateTime time.Duration
func ClientReceive(sock *net.Socket, playerData *PlayerData, networkChannel chan serdes.WorldUpdate) error {
	// lastWorldUpdate := time.Now()
	bufLen := 100
	worldUpdateTimes := ds.NewRingBuffer[time.Duration](bufLen)
	for i := 0; i < bufLen; i++ {
		worldUpdateTimes.Add(4 * mmo.FixedTimeStep) // TODO! - hardcoded
	}

	for {
		msg, err := sock.Recv()
		if errors.Is(err, net.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			log.Warn().Err(err).Msg("ClientReceive NetworkErr")
			return err
		} else if errors.Is(err, net.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			log.Error().Err(err).Msg("ClientReceive SerdesErr")
			continue
		}
		if msg == nil { continue }

		switch t := msg.(type) {
		case serdes.WorldUpdate:
			// log.Print("Ticks: ", t.Tick, t.PlayerTick)
			// {
			// 	worldUpdateTimes.Add(time.Since(lastWorldUpdate))
			// 	lastWorldUpdate = time.Now()
			// 	buf := worldUpdateTimes.Buffer()
			// 	AvgWorldUpdateTime = 0
			// 	for i := range buf {
			// 		AvgWorldUpdateTime += buf[i]
			// 	}
			// 	AvgWorldUpdateTime = AvgWorldUpdateTime / time.Duration(len(buf))
			// 	// log.Print("AvgWorldUpdateTime: ", AvgWorldUpdateTime)
			// }

			// log.Print("Client-NewWorldUpdate")
			// playerData.SetTicks(t.Tick, t.PlayerTick)

			// Note: Because the client received this speech bubble update from the server, we will handle the HandleSent() so that the client doesn't try to resend it to the server.
			// This code just calls HandleSent() on the player's speech bubble if they just received their own speech bubble
			compSlice, ok := t.WorldData[playerData.Id()]
			if ok {
				newCompSlice := make([]ecs.Component, 0)
				// Pull out mmo.Speech for playerId
				for _, c := range compSlice {
					switch t := c.(type) {
					case ecs.CompBox[mmo.Speech]:
						msg := t.Get().Text
						log.Print("Client received a message for himself! ", msg)
						speech := mmo.Speech{
							Text: msg,
						}
						speech.HandleSent()
						// TODO - speech.HandleRender() - Would I ever use this to have the server send messages to the client?
						// compSlice[i] = ecs.C(speech)
						newCompSlice = append(newCompSlice, ecs.C(speech))
					case ecs.CompBox[mmo.Input]:
						// If the server sent us back our own input, we just want to drop it, because we own that component
						continue
					default:
						newCompSlice = append(newCompSlice, c)
					}
				}
				t.WorldData[playerData.Id()] = newCompSlice
			}

			for j, compSlice := range t.WorldData {
				for i, c := range compSlice {
					switch tt := c.(type) {
					case ecs.CompBox[phy2.Pos]:
						serverTransform := ServerTransform{
							Pos: tt.Get(),
							Handled: false,
							ServerTick: t.Tick,
							PlayerTick: t.PlayerTick,
						}
						compSlice[i] = ecs.C(serverTransform)
						// nextTransform := NextTransform{
						// 	PhyTrans: t.Get(),
						// 	Replayed: false,
						// }
						// serverTransform := ServerTransform(t.Get())
						// compSlice[i] = ecs.C(nextTransform)
						// compSlice = append(compSlice, ecs.C(serverTransform))
					}
				}
				t.WorldData[j] = compSlice

				// for i := range compSlice {
				// 	log.Printf("%T\n", compSlice[i])
				// }
			}

			networkChannel <- t
		case serdes.ClientLoginResp:
			log.Print("serdes.ClientLoginResp", t)
			// TODO this might be needed in the future if I want to write any data on login resp
			// ecs.Write(world, ecs.Id(t.Id), ecs.C(mmo.Body{}))
			// networkChannel <- serdes.WorldUpdate{
			// 	UserId: t.UserId,
			// 	WorldData: map[ecs.Id][]ecs.Component{
			// 		ecs.Id(t.Id): []ecs.Component{
			// 			ecs.C(mmo.Body{}),
			// 		},
			// 	},
			// }

			playerData.SetId(t.Id)

			networkChannel <- serdes.WorldUpdate{
				UserId: t.UserId,
				WorldData: map[ecs.Id][]ecs.Component{
					ecs.Id(t.Id): []ecs.Component{
						ecs.C(mmo.Input{}),
						ecs.C(phy2.Pos{}),
						ecs.C(Keybinds{
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
