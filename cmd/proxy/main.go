package main

import (
	"log"
	"os"
	"os/signal"
	"net/http"
	"sync"
	"net"
	"time"
	"context"
	"encoding/json"

	"nhooyr.io/websocket"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"

	"github.com/jstewart7/mmo"
	"github.com/jstewart7/mmo/engine/physics"
)

func main() {
	url := "tcp://127.0.0.1:9000"

	sock, err := pair.NewSocket()
	if err != nil {
		panic(err)
	}

	err = sock.Dial(url)
	if err != nil {
		panic(err)
	}

	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		panic(err)
	}

	room := NewRoom()

	go room.HandleGameUpdates(sock)

	s := &http.Server{
		Handler: websocketServer{
			serverSocket: sock,
			room: room,
		},
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Println("Starting Proxy", listener.Addr())

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

	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()

	err = s.Shutdown(ctx)
	if err != nil {
		log.Println("Error shutting down server:", err)
	}
}

type websocketServer struct {
	serverSocket mangos.Socket
	room *Room
}

func (s websocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println("Error Accepting Websocket:", err)
		return
	}

	ctx := context.Background()

	conn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	go ServeNetConn(conn, s.serverSocket, s.room)
}

func ServeNetConn(conn net.Conn, serverSocket mangos.Socket, room *Room) {
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
	username := "username"
	room.mu.Lock()
	_, ok := room.Map[username]
	if ok {
		log.Println("Duplicate Login Detected! Exiting.")
		room.mu.Unlock()
		return
	}
	room.Map[username] = conn
	room.mu.Unlock()
	defer func() {
		room.mu.Lock()
		delete(room.Map, username)
		room.mu.Unlock()
	}()

	loginMessage := mmo.ProxyToServerMessage{
		Type: "login",
		Username: username,
	}
	serLogin, err := json.Marshal(loginMessage)
	if err != nil {
		log.Println("Failed to serialize", loginMessage)
		return
	}

	err = serverSocket.Send(serLogin)
	if err != nil {
		log.Println("Failed to send login message")
		return
	}

	// Read data from client and sends to game server
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

			inputMessage := mmo.ProxyToServerMessage{
				Type: "input",
				Username: username,
				Input: input,
			}

			serInput, err := json.Marshal(inputMessage)
			if err != nil {
				log.Println("Failed to serialize", inputMessage)
				return
			}

			err = serverSocket.Send(serInput)
			if err != nil {
				log.Println("Failed to send input message")
				return
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

// TODO - rename
type Room struct {
	mu sync.RWMutex
	Map map[string]net.Conn
}

func NewRoom() *Room {
	return &Room{
		Map: make(map[string]net.Conn),
	}
}

func (r *Room) HandleGameUpdates(sock mangos.Socket) {
	// Read data from game server and send to client
	for {
		msg, err := sock.Recv()
		if err != nil {
			log.Println("Read Error:", err)
		}

		proxyMsg := mmo.ServerToProxyMessage{}
		err = json.Unmarshal(msg, &proxyMsg)
		if err != nil {
			log.Println("Message didn't match input:", msg)
			continue
		}

		username := proxyMsg.Username
		updateMsg := proxyMsg.Update

		switch proxyMsg.Type {
		case "physics":
			r.mu.RLock()
			conn, ok := r.Map[username]
			r.mu.RUnlock()
			if ok {
				serializedUpdate, err := json.Marshal(updateMsg)
				if err != nil {
					log.Println("Failed to serialize", updateMsg)
					continue
				}

				_, err = conn.Write(serializedUpdate)
				if err != nil {
					log.Println("Error Sending:", err)
					// TODO - User disconnected? Remove from map?
				}
			} else {
				log.Println("User Disconnected", username)
			}
		}
	}
}
