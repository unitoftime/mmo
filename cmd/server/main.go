package main

import (
	"log"
	"os"
	"os/signal"

	"go.nanomsg.org/mangos/v3/protocol/pair"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"

	"github.com/unitoftime/ecs"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
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

	serverConn := mmo.ServerConn{
		Encoder: serdes.New(),
		Sock: sock,
	}

	// Load Game
	engine := ecs.NewWorld()
	_ = mmo.LoadGame(engine)

	// This is the list of entities to get deleted
	// TODO - make this thread safe
	deleteList := mmo.NewDeleteList()

	// TODO - make configurable
	networkChannel := make(chan serdes.WorldUpdate, 1024)

	serverSystems := mmo.CreateServerSystems(engine, serverConn, networkChannel, deleteList)

	quit := ecs.Signal{}
	quit.Set(false)

	go ecs.RunGameFixed(serverSystems, &quit)

	go mmo.ServeProxyConnection(serverConn, engine, networkChannel, deleteList)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select{
	case sig := <-sigs:
		log.Println("Terminating:", sig)
	}
	quit.Set(true)
}
