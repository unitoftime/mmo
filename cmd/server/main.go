package main

import (
	"log"
	"os"
	"os/signal"
	"encoding/json"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"

	"github.com/jstewart7/mmo"
	"github.com/jstewart7/mmo/engine/ecs"
	"github.com/jstewart7/mmo/engine/physics"
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
	engine := ecs.NewEngine()
	_ = mmo.LoadGame(engine)

	// TODO - make configurable
	networkChannel := make(chan mmo.ChannelUpdate, 1024)
	serverSystems := mmo.CreateServerSystems(engine, sock, networkChannel)

	quit := ecs.Signal{}
	quit.Set(false)

	go ecs.RunGameFixed(serverSystems, &quit)

	go ServeProxyConnection(sock, engine, networkChannel)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select{
	case sig := <-sigs:
		log.Println("Terminating:", sig)
	}
	quit.Set(true)
}

func ServeProxyConnection(sock mangos.Socket, engine *ecs.Engine, networkChannel chan mmo.ChannelUpdate) {
	log.Println("Server: ServeProxyConnection")
	loginMap := make(map[string]ecs.Id)

	// Read data
	for {
		msg, err := sock.Recv()
		if err != nil {
			log.Println("Read Error:", err)
		}

		proxyMsg := mmo.ProxyToServerMessage{}
		err = json.Unmarshal(msg, &proxyMsg)
		if err != nil {
			log.Println("Message didn't match input:", msg)
			continue
		}

		switch proxyMsg.Type {
		case "login":
			// Login player
			// TODO - put into a function
			username := proxyMsg.Username

			id := engine.NewId()
			ecs.Write(engine, id, mmo.User{username})
			ecs.Write(engine, id, physics.Input{})
			ecs.Write(engine, id, mmo.SpawnPoint())
			log.Println("Logging in player:", id)

			loginMap[username] = id

		case "input":
			id := loginMap[proxyMsg.Username]
			input := proxyMsg.Input

			networkChannel <- mmo.ChannelUpdate{
				Id: id,
				Component: input,
			}
		}
	}
}
