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

	"nhooyr.io/websocket"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	_ "go.nanomsg.org/mangos/v3/transport/tcp"

	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/ecs"
)

func main() {
	url := "tcp://127.0.0.1:9000"

	sock, err := pair.NewSocket()
	if err != nil {
		panic(err)
	}

	for {
		err = sock.Dial(url)
		if err != nil {
			log.Println("Failed to dial, retrying...")
			time.Sleep(10 * time.Second)
			continue
		}

		break // If we get here, then we've successfully dialed
	}

	serverConn := ServerConnection{
		encoder: serdes.New(),
		sock: sock,
	}

	listener, err := net.Listen("tcp", ":8001")
	if err != nil {
		panic(err)
	}

	room := NewRoom()

	go room.HandleGameUpdates(serverConn)

	s := &http.Server{
		Handler: websocketServer{
			serverConn: serverConn,
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

type ServerConnection struct {
	encoder *serdes.Serdes
	sock mangos.Socket
}

type ClientConnection struct {
	encoder *serdes.Serdes
	conn net.Conn
}

type websocketServer struct {
	serverConn ServerConnection
	room *Room
}

func (s websocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:8081"}, // TODO - Refactor this once I have a good deployment format
	})
	if err != nil {
		log.Println("Error Accepting Websocket:", err)
		return
	}

	ctx := context.Background()

	conn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	go ServeNetConn(conn, s.serverConn, s.room)
}

// This is just to make sure different users get different login ids
var userIdCounter uint64

// Handles the websocket connection to a specific client in the room
func ServeNetConn(conn net.Conn, serverConn ServerConnection, room *Room) {
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
	const MaxMsgSize int = 4 * 1024 // TODO - use the maximum MTU size

	// Login player
	room.mu.Lock()
	// TODO - Eventually This id should come from the login request which probably has a JWT which encodes the data. You probably don't need that in a lock
	userId := userIdCounter
	userIdCounter++
	_, ok := room.Map[userId]
	if ok {
		log.Println("Duplicate Login Detected! Exiting.")
		room.mu.Unlock()
		return
	}

	clientEncoder := serdes.New()
	room.Map[userId] = ClientConnection{
		conn: conn,
		encoder: clientEncoder,
	}
	room.mu.Unlock()

	// Cleanup room once they leave
	defer func() {
		room.mu.Lock()
		delete(room.Map, userId)
		room.mu.Unlock()
	}()

	// Send login message to server
	log.Println("Sending Login Message for", userId)
	// serLogin := serdes.MarshalClientLoginMessage(userId)
	serLogin, err := serverConn.encoder.Marshal(serdes.ClientLogin{userId})
	if err != nil {
		log.Println("Failed to marshal message", err)
		return
	}
	log.Println("ProxyServeNetConn:", len(serLogin))

	err = serverConn.sock.Send(serLogin)
	if err != nil {
		log.Println("Failed to send login message", err)
		return
	}

	// Send logout message to server
	defer func() {
		// TODO - maybe just kick of a goroutine that just continually tries to do this until it succeeds. Or maybe have one worker that reads from a queue or map or something like that
		// serLogout := serdes.MarshalClientLogoutMessage(userId)
		serLogout, err := serverConn.encoder.Marshal(serdes.ClientLogout{userId})
		if err != nil {
			log.Println("Failed to marshal message", err)
		}
		log.Println("ProxyServeNetConn:", len(serLogout))

		err = serverConn.sock.Send(serLogout)
		if err != nil {
			panic("Failed to send logout message") // TODO - this needs to not panic
		}
	}()

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

			// log.Println("Unmarshalling")
			// TODO - replace with mutateInPlace code?
			// fbMessage, err := serdes.UnmarshalMessage(msg)
			// Note: Slice off msg buffer based on how many bytes we actually read
			fbMessage, err := clientEncoder.Unmarshal(msg[:n])
			if err != nil {
				log.Println("Failed to unmarshal:", err)
			}
			log.Println("ServeNetConn:", fbMessage)

			switch t := fbMessage.(type) {
			case serdes.WorldUpdate:
				log.Println("Client->Proxy: World Update received")
				// TODO - replace with mutateInPlace code?
				t.UserId = userId
				serializedUpdate, err := serverConn.encoder.Marshal(t)
				if err != nil {
					log.Println("Error Marshalling", err)
					continue
				}
				log.Println("ProxyServeNetConn:", len(serializedUpdate))

				err = serverConn.sock.Send(serializedUpdate)
				if err != nil {
					log.Println("Error Sending:", err)
				}
			default:
				panic("Unknown message type")
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
	Map map[uint64]ClientConnection
}

func NewRoom() *Room {
	return &Room{
		Map: make(map[uint64]ClientConnection),
	}
}

// Read data from game server and send to client
func (r *Room) HandleGameUpdates(serverConn ServerConnection) {
	for {
		msg, err := serverConn.sock.Recv()
		if err != nil {
			log.Println("Read Error:", err)
		}

		fbMessage, err := serverConn.encoder.Unmarshal(msg)
		if err != nil {
			log.Println("Failed to unmarshal:", err)
		}
		// log.Println("HandleGameUpdate:", fbMessage)

		switch t := fbMessage.(type) {
		case serdes.WorldUpdate:
			r.mu.RLock()
			clientConn, ok := r.Map[t.UserId]
			r.mu.RUnlock()
			if ok {
				// TODO - replace with mutateInPlace code?
				t.UserId = 0
				serializedUpdate, err := clientConn.encoder.Marshal(t)
				if err != nil {
					log.Println("Error Marshalling", err)
					continue
				}
				log.Println("Proxy WorldUpdate:", t)

				_, err = clientConn.conn.Write(serializedUpdate)
				if err != nil {
					log.Println("Error Sending:", err)
					// TODO - User disconnected? Remove from map?
				}
			} else {
				log.Println("User Disconnected", t.UserId)
				// TODO - Send back to server "hey this person disconnected!"
			}
		case serdes.ClientLoginResp:
			log.Println("serdes.ClientLoginResp")
			r.mu.RLock()
			clientConn, ok := r.Map[t.UserId]
			r.mu.RUnlock()
			if ok {
				// TODO - replace with mutateInPlace code?
				t.UserId = 0
				// serializedMsg, err := serdes.MarshalClientLoginRespMessage(t.UserId, ecs.Id(t.Id))
				serializedMsg, err := clientConn.encoder.Marshal(serdes.ClientLoginResp{t.UserId, ecs.Id(t.Id)})
				if err != nil {
					log.Println("Error Marshalling", err)
					continue
				}
				log.Println("Proxy LoginResp:", t)

				_, err = clientConn.conn.Write(serializedMsg)
				if err != nil {
					log.Println("Error Sending:", err)
					// TODO - User disconnected? Remove from map?
				}
			} else {
				log.Println("User Disconnected", t.UserId)
				// TODO - Send back to server "hey this person disconnected!"
			}
		case serdes.ClientLogoutResp:
			log.Println("serdes.ClientLogoutResp")
			// TODO - should I double check that they've been removed from the map?
			// TODO - I should send a "logged out successful" message
		default:
			panic("Unknown message type")
		}
	}
}
