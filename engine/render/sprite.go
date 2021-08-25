package render

import (
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"

	"github.com/jstewart7/mmo/engine/ecs"
	"github.com/jstewart7/mmo/engine/physics"
)

type Sprite struct {
	Position pixel.Vec
	*pixel.Sprite
}
func (t *Sprite) ComponentSet(val interface{}) { *t = val.(Sprite) }

type Keybinds struct {
	Up, Down, Left, Right pixelgl.Button
}
func (t *Keybinds) ComponentSet(val interface{}) { *t = val.(Keybinds) }

// Note: val should probably be between 0 and 1
func Interpolate(A, B pixel.Vec, lowerBound, upperBound float64) pixel.Vec {
	delta := B.Sub(A)
	dMag := delta.Len()

	interpValue := 0.0
	if dMag > upperBound {
		interpValue = 1.0
	} else if dMag > lowerBound {
		// y - y1 = m(x - x1)
		slope := 1/(upperBound - lowerBound)
		interpValue = slope * (dMag - lowerBound) + 0
	}

	deltaScaled := delta.Scaled(interpValue)
	return A.Add(deltaScaled)
}

// TODO - interpolate based off of the time till the next fixedTimeStep?
func InterpolateSpritePositions(engine *ecs.Engine, dt time.Duration) {
	ecs.Each(engine, Sprite{}, func(id ecs.Id, a interface{}) {
		sprite := a.(Sprite)

		transform := physics.Transform{}
		ok := ecs.Read(engine, id, &transform)
		if !ok { return }
		physicsPosition := pixel.V(transform.X, transform.Y)

		// TODO - make configurable
		// sprite.Position = physicsPosition
		sprite.Position = Interpolate(sprite.Position, physicsPosition, 1.0, 16.0)
		ecs.Write(engine, id, sprite)
	})
}

func DrawSprites(win *pixelgl.Window, engine *ecs.Engine) {
	ecs.Each(engine, Sprite{}, func(id ecs.Id, a interface{}) {
		sprite := a.(Sprite)
		sprite.Draw(win, pixel.IM.Scaled(pixel.ZV, 2.0).Moved(sprite.Position))
	})
}

func CaptureInput(win *pixelgl.Window, engine *ecs.Engine) {
	ecs.Each(engine, Keybinds{}, func(id ecs.Id, a interface{}) {
		keybinds := a.(Keybinds)

		input := physics.Input{}
		ok := ecs.Read(engine, id, &input)
		if !ok { return }

		input.Left = false
		input.Right = false
		input.Up = false
		input.Down = false

		if win.Pressed(keybinds.Left) {
			input.Left = true
		}
		if win.Pressed(keybinds.Right) {
			input.Right = true
		}
		if win.Pressed(keybinds.Up) {
			input.Up = true
		}
		if win.Pressed(keybinds.Down) {
			input.Down = true
		}

		ecs.Write(engine, id, input)
	})
}
