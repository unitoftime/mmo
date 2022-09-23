package main

import (
	"log"
	"os"
	"os/signal"

	// "go.nanomsg.org/mangos/v3/protocol/pair"
	// _ "go.nanomsg.org/mangos/v3/transport/tcp"

	"github.com/unitoftime/ecs"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Start the networking layer
	url := "127.0.0.1:9000"
	log.Println("Starting Server", url)

	// Load Game
	world := ecs.NewWorld()
	_ = mmo.LoadGame(world)

	// This is the list of entities to get deleted
	deleteList := mmo.NewDeleteList()

	// TODO - make configurable
	networkChannel := make(chan serdes.WorldUpdate, 1024)

	server := mmo.NewServer(url, func(conn mmo.ServerConn) error {
		return mmo.ServeProxyConnection(conn, world, networkChannel, deleteList)
	})

	serverSystems := mmo.CreateServerSystems(world, server, networkChannel, deleteList)

	quit := ecs.Signal{}
	quit.Set(false)

	go ecs.RunGameFixed(serverSystems, &quit)

	// go mmo.ServeProxyConnection(serverConn, engine, networkChannel, deleteList)
	go server.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select{
	case sig := <-sigs:
		log.Println("Terminating:", sig)
	}
	quit.Set(true)
}
