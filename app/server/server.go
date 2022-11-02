package server

import (
	"os"
	"os/signal"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

func Main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Start the networking layer
	url := "127.0.0.1:9000"
	log.Print("Starting Server", url)

	// Load Game
	world := ecs.NewWorld()
	tilemap := mmo.LoadGame(world)

	// This is the list of entities to get deleted
	deleteList := mmo.NewDeleteList()

	// TODO - make configurable
	networkChannel := make(chan serdes.WorldUpdate, 1024)

	server, err := mmo.NewServer(url, func(conn *mmo.ServerConn) error {
		return mmo.ServeProxyConnection(conn, world, networkChannel, deleteList)
	})
	if err != nil {
		panic(err)
	}

	serverSystems := mmo.CreateServerSystems(world, server, networkChannel, deleteList, tilemap)

	quit := ecs.Signal{}
	quit.Set(false)

	go ecs.RunGameFixed(serverSystems, &quit)

	go server.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select{
	case sig := <-sigs:
		log.Print("Terminating:", sig)
	}
	quit.Set(true)
}
