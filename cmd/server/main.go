package main

import (
	"os"
	"os/signal"
	"log"
	"time"
	"net"
	"net/http"
	"context"
	"encoding/json"

	"nhooyr.io/websocket"

	"github.com/jstewart7/mmo"
	"github.com/jstewart7/mmo/engine/ecs"
	"github.com/jstewart7/mmo/engine/physics"
)

func main() {
	// Load Game
	engine := ecs.NewEngine()
	_ = mmo.LoadGame(engine)

	// TODO - make configurable
	networkChannel := make(chan mmo.ChannelUpdate, 1024)
	physicsSystems := mmo.CreateServerSystems(engine)

	quit := ecs.Signal{}
	quit.Set(false)

	inputSystems := []ecs.System{
		mmo.CreatePollNetworkSystem(engine, networkChannel),
	}

	go ecs.RunGame(inputSystems, physicsSystems, []ecs.System{}, &quit)

	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		panic(err)
	}

	s := &http.Server{
		Handler: websocketServer{
			engine: engine,
			networkChannel: networkChannel,
		},
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("Starting Server", listener.Addr())

	errc := make(chan error, 1)
	go func() {
		errc <- s.Serve(listener)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	select{
	case err := <-errc:
		log.Println("Failed to serve:", err)
	case sig := <-sigs:
		log.Println("Terminating:", sig)
	}

	quit.Set(true)

	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()

	err = s.Shutdown(ctx)
	if err != nil {
		log.Println("Error shutting down server:", err)
	}
}

type websocketServer struct {
	engine *ecs.Engine
	networkChannel chan mmo.ChannelUpdate
}

func (s websocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println("Error Accepting Websocket:", err)
		return
	}

	ctx := context.Background()

	conn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	go ServeNetConn(s.engine, conn, s.networkChannel)
}

func ServeNetConn(engine *ecs.Engine, conn net.Conn, networkChannel chan mmo.ChannelUpdate) {
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Println("Error Closing net.Conn:", err)
		}
	}()

	timeoutSeconds := 60 * time.Second
	timeout := make(chan uint8, 1)
	const StopTimeout uint8 = 0
	const ContTimeout uint8 = 1
	const MaxMsgSize int = 4 * 1024

	// Login player
	// TODO - put into a function
	id := engine.NewId()
	ecs.Write(engine, id, mmo.Websocket{conn})
	ecs.Write(engine, id, physics.Input{})
	ecs.Write(engine, id, mmo.SpawnPoint())
	log.Println("Logging in player:", id)

	// Read data
	// TODO - TCP doesn't provide framing, so message framing needs to be added
	// TODO - TCP will send 0 byte messages to indicate closes, websockets sends them without closing
	go func() {
		msg := make([]byte, MaxMsgSize)
		for {
			n, err := conn.Read(msg)

			if err != nil {
				log.Println("Read Error:", err)
				timeout <- StopTimeout // Stop timeout because of a read error
				return
			} else if n <= 0 {
				continue
			}

			// Tick the timeout watcher so we don't timeout!
			timeout <- ContTimeout

			// TODO - handle multiple message types
			input := physics.Input{}
			err = json.Unmarshal(msg[:n], &input)
			if err != nil {
				log.Println("Message didn't match input:", msg[:n])
				continue
			}

			networkChannel <- mmo.ChannelUpdate{
				Id: id,
				Component: input,
			}
		}
	}()

	// Manage Timeout
ExitTimeout:
	for {
		select {
		case res := <-timeout:
			if res == StopTimeout {
				log.Println("Manually Stopping Timeout Manager")
				break ExitTimeout
			}
		case <-time.After(timeoutSeconds):
			log.Println("User timed out!")
			break ExitTimeout
		}
	}
}
