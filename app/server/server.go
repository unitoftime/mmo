package server

import (
	"os"
	"os/signal"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/net"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

func Main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Load Game
	world := ecs.NewWorld()
	tilemap := mmo.LoadGame(world)

	// This is the list of entities to get deleted
	deleteList := NewDeleteList()

	// TODO - make configurable
	networkChannel := make(chan serdes.WorldUpdate, 1024)

	// Start the networking layer
	url := "tcp://127.0.0.1:9000"
	log.Print("Starting Server", url)
	serverNet := net.Config{
		Url: url,
		Serdes: serdes.New(),
	}
	listener, err := serverNet.Listen()
	if err != nil {
		panic(err)
	}

	server := NewServer(listener, func(conn *ServerConn) error {
		return ServeProxyConnection(conn, world, networkChannel, deleteList)
	})

	serverSystems := CreateServerSystems(world, server, networkChannel, deleteList, tilemap)

	quit := ecs.Signal{}
	quit.Set(false)

	schedule := mmo.GetScheduler()
	schedule.AppendPhysics(serverSystems...)
	go schedule.Run(&quit)

	// go ecs.RunGameFixed(serverSystems, &quit)

	go server.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select{
	case sig := <-sigs:
		log.Print("Terminating:", sig)
	}
	quit.Set(true)
}
