package physics

import (
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

func HandleInput(engine *ecs.Engine) {
	ecs.Each(engine, Input{}, func(id ecs.Id, a interface{}) {
		input := a.(Input)

		transform := Transform{}
		ok := ecs.Read(engine, id, &transform)
		if !ok { return }

		if input.Left {
			transform.X -= 2.0
		}
		if input.Right {
			transform.X += 2.0
		}
		if input.Up {
			transform.Y += 2.0
		}
		if input.Down {
			transform.Y -= 2.0
		}

		ecs.Write(engine, id, transform)
	})
}
