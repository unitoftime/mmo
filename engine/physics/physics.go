package physics

import (
	"time"
	"github.com/jstewart7/ecs"
)

type Transform struct {
	X, Y float64
}

type Input struct {
	Up, Down, Left, Right bool
}

func HandleInput(world *ecs.World, dt time.Duration) {
	view := ecs.ViewAll(world, &Input{}, &Transform{})
	view.Map(func(id ecs.Id, comp ...interface{}) {
		input := comp[0].(*Input)
		transform := comp[1].(*Transform)
		// ecs.Each(engine, Input{}, func(id ecs.Id, a interface{}) {
		// input := a.(Input)
		// // Note: 100 good starting point, 200 seemed like a good max
		// speed := 125.0
		// transform := Transform{}
		// ok := ecs.Read(engine, id, &transform)
		// if !ok { return }

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

		// ecs.Write(engine, id, transform)
	})
}
