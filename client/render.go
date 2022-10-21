package client

import (
	"time"
	"github.com/unitoftime/glitch"
	"github.com/unitoftime/ecs"
	"github.com/unitoftime/mmo/game"
	"github.com/unitoftime/flow/render"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/flow/asset"
)

type SpeechRender struct {
	Text *glitch.Text
	RemainingDuration time.Duration
}

func SetSpeech(world *ecs.World, atlas *glitch.Atlas, id ecs.Id, message string) {
	message = game.FilterChat(message)

	ecs.Write(world, id,
		ecs.C(game.Speech{
			Text: message,
			// handled: false,
		}),
		ecs.C(SpeechRender{
			Text: atlas.Text(message),
			RemainingDuration: 5 * time.Second, // TODO - should this scale based on text length? @Conifer's Idea wow!!!!
		}),
	)
}


type Animation struct {
	Body *render.Animation
	Hat *render.Animation
}

func (a *Animation) SetAnimation(name string) {
	a.Body.SetAnimation(name)
	if a.Hat != nil {
		a.Hat.SetAnimation(name)
	}
}

func PlayAnimations(pass *glitch.RenderPass, world *ecs.World, dt time.Duration) {
	ecs.Map2(world, func(id ecs.Id, anim *Animation, t *physics.Transform) {
		anim.Body.Update(dt)
		anim.Hat.Update(dt)

		anim.Body.Draw(pass, t)

		hatPoint := *t
		// hatPoint.X += float64(anim.Offset[0])
		// hatPoint.Y += float64(anim.Offset[1])
		frame := anim.Body.GetFrame()
		mountPoint, ok := frame.Mount["hat"]
		if ok {
			hatPoint.X += float64(mountPoint[0])
			hatPoint.Y += float64(mountPoint[1])
		}
		anim.Hat.Draw(pass, &hatPoint)
	})
}

func mirrorAnim(anims map[string][]render.Frame, from, to string) {
	mirroredAnim := make([]render.Frame, 0)
	for i, frame := range anims[from] {
		mirroredAnim = append(mirroredAnim, frame)
		mirroredAnim[i].MirrorY = true // TODO - this should probably be !MirrorY
	}
	anims[to] = mirroredAnim
}

func loadAnim(animAssets *asset.Animation) map[string][]render.Frame {
	manFrames := make(map[string][]render.Frame)
	for animName, frames := range animAssets.Frames {
		renderFrames := make([]render.Frame, 0)
		for _, frame := range frames {
			rFrame := render.NewFrame(frame.Sprite, frame.Duration)
			rFrame.MirrorY = frame.MirrorY
			renderFrames = append(renderFrames, rFrame)
		}
		manFrames[animName] = renderFrames
	}
	return manFrames
}

func NewAnimation(load *asset.Load, spritesheet *asset.Spritesheet, body game.Body) Animation {
	manAssets, err := load.AseAnimation(spritesheet, "assets/man.json")
	if err != nil {
		panic(err)
	}
	manFrames := loadAnim(manAssets)
	mirrorAnim(manFrames, "run_left", "run_right")
	mirrorAnim(manFrames, "idle_left", "idle_right")
	bodyAnim := render.NewAnimation("idle_left", manFrames)

	hatAssets, err := load.AseAnimation(spritesheet, "assets/hat-top.json")
	if err != nil {
		panic(err)
	}
	hatFrames := loadAnim(hatAssets)
	mirrorAnim(hatFrames, "run_left", "run_right")
	mirrorAnim(hatFrames, "idle_left", "idle_right")
	hatAnim := render.NewAnimation("idle_left", hatFrames)

	// idleAnim[0].Mount["hat"] = glitch.Vec2{0, 7}
	// idleAnim[1].Mount["hat"] = glitch.Vec2{0, 6}
	// idleAnim[2].Mount["hat"] = glitch.Vec2{0, 5}
	// idleAnim[3].Mount["hat"] = glitch.Vec2{0, 6}
	// runAnim[0].Mount["hat"] = glitch.Vec2{0, 8}
	// runAnim[1].Mount["hat"] = glitch.Vec2{0, 9}
	// runAnim[2].Mount["hat"] = glitch.Vec2{0, 8}
	// runAnim[3].Mount["hat"] = glitch.Vec2{0, 7}
	// runRightAnim[0].Mount["hat"] = glitch.Vec2{0, 8}
	// runRightAnim[1].Mount["hat"] = glitch.Vec2{0, 9}
	// runRightAnim[2].Mount["hat"] = glitch.Vec2{0, 7}
	// runRightAnim[3].Mount["hat"] = glitch.Vec2{0, 7}

	return Animation{
		Body: &bodyAnim,
		Hat: &hatAnim,
	}
}
