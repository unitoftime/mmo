package main

import (
	"os"
	"image"
	_ "image/png"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
)

func main() {
	pixelgl.Run(runGame)
}

func runGame() {
	cfg := pixelgl.WindowConfig{
		Title: "MMO",
		Bounds: pixel.R(0, 0, 1024, 768),
		VSync: true,
		Resizable: true,
	}

	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	win.SetSmooth(false)

	manSprite, err := getSprite("man.png")
	if err != nil {
		panic(err)
	}
	manPosition := win.Bounds().Center()

	for !win.JustPressed(pixelgl.KeyEscape) {
		win.Clear(pixel.RGB(0, 0, 0))

		if win.Pressed(pixelgl.KeyLeft) {
			manPosition.X -= 2.0
		}
		if win.Pressed(pixelgl.KeyRight) {
			manPosition.X += 2.0
		}
		if win.Pressed(pixelgl.KeyUp) {
			manPosition.Y += 2.0
		}
		if win.Pressed(pixelgl.KeyDown) {
			manPosition.Y -= 2.0
		}

		manSprite.Draw(win, pixel.IM.Scaled(pixel.ZV, 2.0).Moved(manPosition))

		win.Update()
	}
}

func getSprite(path string) (*pixel.Sprite, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	pic := pixel.PictureDataFromImage(img)

	return pixel.NewSprite(pic, pic.Bounds()), nil
}
