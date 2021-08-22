package physics

import (
	"time"
	"github.com/jstewart7/mmo/engine/ecs"
)

type Transform struct {
	X, Y float64
}
func (t *Transform) ComponentSet(val interface{}) { *t = val.(Transform) }

type Input struct {
	Up, Down, Left, Right bool
}
func (t *Input) ComponentSet(val interface{}) { *t = val.(Input) }

func HandleInput(engine *ecs.Engine, dt time.Duration) {
	ecs.Each(engine, Input{}, func(id ecs.Id, a interface{}) {
		input := a.(Input)

		speed := 100.0

		transform := Transform{}
		ok := ecs.Read(engine, id, &transform)
		if !ok { return }

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

		ecs.Write(engine, id, transform)
	})
}
