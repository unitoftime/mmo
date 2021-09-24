package main

import (
	"log"
	"os"
	"os/signal"

	"go.nanomsg.org/mangos/v3/protocol/pair"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"

	"github.com/jstewart7/ecs"

	"github.com/jstewart7/mmo"
	"github.com/jstewart7/mmo/serdes"
)

func main() {
	// Start the networking layer
	url := "tcp://127.0.0.1:9000"
	log.Println("Starting Mangos Server", url)

	sock, err := pair.NewSocket()
	if err != nil {
		panic(err)
	}

	err = sock.Listen(url)
	if err != nil {
		panic(err)
	}

	// Load Game
	engine := ecs.NewWorld()
	_ = mmo.LoadGame(engine)

	// TODO - make configurable
	networkChannel := make(chan serdes.WorldUpdate, 1024)
	serverSystems := mmo.CreateServerSystems(engine, sock, networkChannel)

	quit := ecs.Signal{}
	quit.Set(false)

	go ecs.RunGameFixed(serverSystems, &quit)

	go mmo.ServeProxyConnection(sock, engine, networkChannel)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select{
	case sig := <-sigs:
		log.Println("Terminating:", sig)
	}
	quit.Set(true)
}
