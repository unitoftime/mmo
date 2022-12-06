package client

import (
	"time"
	"github.com/unitoftime/glitch"
	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/render"
	"github.com/unitoftime/flow/phy2"
	"github.com/unitoftime/flow/asset"

	"github.com/unitoftime/packer" // TODO - move packer to flow?

	"github.com/unitoftime/mmo"
)

type SpeechRender struct {
	Text *glitch.Text
	RemainingDuration time.Duration
}

func SetSpeech(world *ecs.World, atlas *glitch.Atlas, id ecs.Id, message string) {
	message = mmo.FilterChat(message)

	ecs.Write(world, id,
		ecs.C(mmo.Speech{
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
	Direction string // indicates if we are going left or right
	Body *render.Animation
	Hat *render.Animation
	batch *glitch.Batch
}

func (a *Animation) SetAnimation(name string) {
	a.Body.SetAnimation(name)
	if a.Hat != nil {
		a.Hat.SetAnimation(name)
	}
}

func PlayAnimations(pass *glitch.RenderPass, world *ecs.World, dt time.Duration) {
	ecs.Map2(world, func(id ecs.Id, anim *Animation, pos *phy2.Pos) {
		if anim.batch == nil {
			anim.batch = glitch.NewBatch()
		}

		anim.Body.Update(dt)
		anim.Hat.Update(dt)

		// TODO - minor optimization opportunity: Don't batch every frame, only the frames that change
		anim.batch.Clear()
		anim.Body.Draw(anim.batch, &phy2.Pos{})

		hatPoint := phy2.Pos{}

		frame := anim.Body.GetFrame()
		mountPoint := frame.Mount("hat")
		hatPoint.X += float64(mountPoint[0])
		hatPoint.Y += float64(mountPoint[1])

		hatFrame := anim.Hat.GetFrame()
		hatDestPoint := hatFrame.Mount("dest")
		hatPoint.X -= float64(hatDestPoint[0])
		hatPoint.Y -= float64(hatDestPoint[1])

		anim.Hat.Draw(anim.batch, &hatPoint)

		mat := glitch.Mat4Ident
		mat.Translate(float32(pos.X), float32(pos.Y), 0)
		anim.batch.Draw(pass, mat)
	})
}

func mirrorAnim(anims map[string][]render.Frame, from, to string) {
	// TODO - as a note. Mirrored anims share the same mount points. This might not right/left mountpoints. It probably works fine for centered mount points
	mirroredAnim := make([]render.Frame, 0)
	for i, frame := range anims[from] {
		mirroredAnim = append(mirroredAnim, frame)
		mirroredAnim[i].MirrorY = true // TODO - this should probably be !MirrorY
	}
	anims[to] = mirroredAnim
}

func loadAnim(animAssets *asset.Animation, mountFrames packer.MountFrames) map[string][]render.Frame {
	manFrames := make(map[string][]render.Frame)
	for animName, frames := range animAssets.Frames {
		renderFrames := make([]render.Frame, 0)
		for _, frame := range frames {

			// Build the frame
			rFrame := render.NewFrame(frame.Sprite, frame.Duration)
			rFrame.MirrorY = frame.MirrorY

			// Add on any available mounting data
			mountData, ok := mountFrames.Frames[frame.Name]
			if ok {
				hatPoint, ok := mountData.MountPoints[0xFF0000]
				if ok {
					point := glitch.Vec2{float32(hatPoint.X), float32(hatPoint.Y)}
					rFrame.SetMount("hat", point)
				}

				// Destination Mount Point TODO - rename dest to something better
				destPoint, ok := mountData.MountPoints[0x000000]
				if ok {
					point := glitch.Vec2{float32(destPoint.X), float32(destPoint.Y)}
					rFrame.SetMount("dest", point)
				}
			}

			// Append it
			renderFrames = append(renderFrames, rFrame)
		}
		manFrames[animName] = renderFrames
	}
	return manFrames
}

func NewAnimation(load *asset.Load, spritesheet *asset.Spritesheet, body mmo.Body) Animation {
	mountFrames, err := load.Mountpoints("assets/mountpoints.json")
	if err != nil {
		panic(err)
	}

	// Load the body
	manAssets, err := load.AseAnimation(spritesheet, "assets/man.json")
	if err != nil {
		panic(err)
	}
	manFrames := loadAnim(manAssets, mountFrames)

	mirrorAnim(manFrames, "run_left", "run_right")
	mirrorAnim(manFrames, "idle_left", "idle_right")
	bodyAnim := render.NewAnimation("idle_left", manFrames)


	hats := []string{
		"assets/hat-top.json",
		"assets/hat-mohawk.json",
		"assets/hat-nightcap.json",
		"assets/hat-bycocket.json",
	}
	hatFile := hats[body.Type]
	// Load the hat
	hatAssets, err := load.AseAnimation(spritesheet, hatFile)
	if err != nil {
		panic(err)
	}
	hatFrames := loadAnim(hatAssets, mountFrames)
	mirrorAnim(hatFrames, "run_left", "run_right")
	mirrorAnim(hatFrames, "idle_left", "idle_right")
	hatAnim := render.NewAnimation("idle_left", hatFrames)

	return Animation{
		Direction: "left",
		Body: &bodyAnim,
		Hat: &hatAnim,
	}
}
