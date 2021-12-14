module github.com/unitoftime/mmo

go 1.16

// replace github.com/unitoftime/ecs => /home/jacob/go/src/github.com/unitoftime/ecs
replace github.com/unitoftime/ecs => /home/jacob/go/src/github.com/unitoftime/ecs

replace github.com/unitoftime/flow => /home/jacob/go/src/github.com/unitoftime/flow

replace github.com/unitoftime/glitch => /home/jacob/go/src/github.com/unitoftime/glitch

replace github.com/unitoftime/gl => /home/jacob/go/src/github.com/unitoftime/gl

replace github.com/unitoftime/glfw => /home/jacob/go/src/github.com/unitoftime/glfw

require (
	github.com/faiface/pixel v0.10.0
	github.com/google/flatbuffers v2.0.0+incompatible
	github.com/gopherjs/gopherjs v0.0.0-20211111143520-d0d5ecc1a356 // indirect
	github.com/unitoftime/ecs v0.0.0-20210925153713-7d62c0f35800
	github.com/unitoftime/flow v0.0.0-20211130161715-4a154b33ad99
	github.com/unitoftime/glitch v0.0.0-20211130161613-9bee272d58d0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	go.nanomsg.org/mangos/v3 v3.3.0
	honnef.co/go/js/dom v0.0.0-20210725211120-f030747120f2 // indirect
	nhooyr.io/websocket v1.8.7
)
