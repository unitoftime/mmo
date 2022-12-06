package client

import (
	"github.com/unitoftime/glitch"
	"github.com/unitoftime/ecs"

	"github.com/unitoftime/mmo"
)

type Keybinds struct {
	Up, Down, Left, Right glitch.Key
}

func CaptureInput(win *glitch.Window, world *ecs.World) {
	// TODO - technically this should only run for the player Ids?
	ecs.Map2(world, func(id ecs.Id, keybinds *Keybinds, input *mmo.Input) {
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
	})
}
