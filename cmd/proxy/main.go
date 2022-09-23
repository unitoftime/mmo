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
	"errors"

	"nhooyr.io/websocket"

	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/mmo/mnet"
	"github.com/unitoftime/ecs"
	// "github.com/unitoftime/mmo"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	url := "tcp://127.0.0.1:9000"

	// sock, err := pair.NewSocket()
	// if err != nil {
	// 	panic(err)
	// }

	// for {
	// 	err = sock.Dial(url)
	// 	if err != nil {
	// 		log.Println("Failed to dial, retrying...")
	// 		time.Sleep(10 * time.Second)
	// 		continue
	// 	}

	// 	break // If we get here, then we've successfully dialed
	// }

	// conn, err := mnet.NewSocket(url)
	// if err != nil { panic(err) }

	// for {
	// 	err := conn.Dial()
	// 	if err != nil {
	// 		log.Println("Failed to dial, retrying...")
	// 		time.Sleep(10 * time.Second)
	// 		continue
	// 	}
	// 	break
	// }

	// serverConn := ServerConnection{
	// 	encoder: serdes.New(),
	// 	conn: conn,
	// }

	room := NewRoom()

	sock, err := mnet.NewSocket(url)
	if err != nil {
		panic(err)
	}

	go mnet.ReconnectLoop(sock, func(sock *mnet.Socket) error {
		room.mu.RLock()
		for userId := range room.Map {
			log.Println("Reconnect - Sending Login Message for", userId)
			// err := serverConn.conn.Send(serdes.ClientLogin{userId})
			err := sock.Send(serdes.ClientLogin{userId})
			if err != nil {
				log.Println(err)
			}
		}
		room.mu.RUnlock()

		return room.HandleGameUpdates(sock)
	})
	// go mmo.ReconnectLoop(world, clientConn, &playerId, networkChannel)

	listener, err := net.Listen("tcp", ":8001")
	if err != nil {
		panic(err)
	}

	// go room.HandleGameUpdates(serverConn)

	s := &http.Server{
		Handler: websocketServer{
			serverConn: sock,
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

// type ServerConnection struct {
// 	encoder *serdes.Serdes
// 	// sock mangos.Socket
// 	conn *mnet.Socket
// }

type ClientConnection struct {
	sock *mnet.Socket
	// encoder *serdes.Serdes
	// conn net.Conn
}

type websocketServer struct {
	// serverConn ServerConnection
	serverConn *mnet.Socket
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
// func ServeNetConn(conn net.Conn, serverConn ServerConnection, room *Room) {
func ServeNetConn(conn net.Conn, serverConn *mnet.Socket, room *Room) {
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

	sock := mnet.NewConnectedSocket(conn)
	room.Map[userId] = ClientConnection{sock}

	// clientEncoder := serdes.New()
	// room.Map[userId] = ClientConnection{
	// 	conn: conn,
	// 	encoder: clientEncoder,
	// }
	room.mu.Unlock()

	// Cleanup room once they leave
	defer func() {
		room.mu.Lock()
		delete(room.Map, userId)
		room.mu.Unlock()
	}()

	// Send login message to server
	log.Println("Sending Login Message for", userId)
	// err := serverConn.conn.Send(serdes.ClientLogin{userId})
	err := serverConn.Send(serdes.ClientLogin{userId})
	if err != nil {
		log.Println(err)
		return
	}

	// Send logout message to server
	defer func() {
		// TODO - maybe just kick of a goroutine that just continually tries to do this until it succeeds. Or maybe have one worker that reads from a queue or map or something like that
		// err := serverConn.conn.Send(serdes.ClientLogout{userId})
		err := serverConn.Send(serdes.ClientLogout{userId})
		if err != nil {
			log.Println(err)
		}
	}()

	// Read data from client and sends to game server
	// TODO - TCP doesn't provide framing, so message framing needs to be added
	// TODO - TCP will send 0 byte messages to indicate closes, websockets sends them without closing
	go func() {
		// msg := make([]byte, MaxMsgSize)
		for {
			msg, err := sock.Recv()
			if errors.Is(err, mnet.ErrNetwork) {
				timeout <- StopTimeout // Stop timeout because of a read error
				log.Println(err)
				return
			} else if errors.Is(err, mnet.ErrSerdes) {
				// Handle errors where we should continue (ie serialization)
				log.Println(err)
				continue
			}

			// Tick the timeout watcher so we don't timeout!
			timeout <- ContTimeout

			// If the message was empty, just continue to the next one
			if msg == nil { continue }

			// n, err := conn.Read(msg)

			// if err != nil {
			// 	log.Println("Read Error:", err)
			// 	timeout <- StopTimeout // Stop timeout because of a read error
			// 	return
			// } else if n <= 0 {
			// 	continue
			// }

			// // Tick the timeout watcher so we don't timeout!
			// timeout <- ContTimeout

			// // log.Println("Unmarshalling")
			// // TODO - replace with mutateInPlace code?
			// // Note: Slice off msg buffer based on how many bytes we actually read
			// fbMessage, err := clientEncoder.Unmarshal(msg[:n])
			// if err != nil {
			// 	log.Println("Failed to unmarshal:", err)
			// }
			// log.Println("ServeNetConn:", fbMessage)

			switch t := msg.(type) {
			case serdes.WorldUpdate:
				log.Println("Client->Proxy: World Update received")
				// TODO - replace with mutateInPlace code?
				t.UserId = userId
				// err := serverConn.conn.Send(t)
				err := serverConn.Send(t)
				if err != nil {
					log.Println(err)
					// TODO - I just continue here, even though we failed to send to the server. I think this is the correct logic, but the proxy needs to retry and connect to the server
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

func (r *Room) GetClientConn(userId uint64) *ClientConnection {
	r.mu.RLock()
	clientConn, ok := r.Map[userId]
	r.mu.RUnlock()
	if !ok {
		log.Println("User Disconnected", userId)
		return nil
	}

	return &clientConn
}

// Read data from game server and send to client
func (r *Room) HandleGameUpdates(serverConn *mnet.Socket) error {
	for {
		// msg, err := serverConn.sock.Recv()
		// if err != nil {
		// 	log.Println("Read Error:", err)
		// }

		// fbMessage, err := serverConn.encoder.Unmarshal(msg)
		// if err != nil {
		// 	log.Println("Failed to unmarshal:", err)
		// }
		// log.Println("HandleGameUpdate:", fbMessage)

		// msg, err := serverConn.conn.Recv()
		msg, err := serverConn.Recv()
		if errors.Is(err, mnet.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			log.Println(err)
			return err
		} else if errors.Is(err, mnet.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			log.Println(err)
			continue
		}
		if msg == nil { continue }
		// if err != nil {
		// 	// TODO - Send back to server "hey this person disconnected!"
		// 	log.Println(err)
		// 	continue
		// }

		switch t := msg.(type) {
		case serdes.WorldUpdate:
			clientConn := r.GetClientConn(t.UserId)
			if clientConn == nil { continue }

			t.UserId = 0
			err := clientConn.sock.Send(t)
			if err != nil {
				log.Println("Error Sending WorldUpdate:", err)
				// TODO - User disconnected? Remove from map?
			}
				// serializedUpdate, err := clientConn.encoder.Marshal(t)
				// if err != nil {
				// 	log.Println("Error Marshalling", err)
				// 	continue
				// }
				// log.Println("Proxy WorldUpdate:", t)

				// _, err = clientConn.conn.Write(serializedUpdate)
				// if err != nil {
				// 	log.Println("Error Sending:", err)
				// 	// TODO - User disconnected? Remove from map?
				// }
		case serdes.ClientLoginResp:
			clientConn := r.GetClientConn(t.UserId)
			if clientConn == nil { continue }

			err := clientConn.sock.Send(serdes.ClientLoginResp{t.UserId, ecs.Id(t.Id)})
			if err != nil {
				log.Println("Error Sending ClientLoginResp:", err)
				// TODO - User disconnected? Remove from map?
			}

			// log.Println("serdes.ClientLoginResp")
			// r.mu.RLock()
			// clientConn, ok := r.Map[t.UserId]
			// r.mu.RUnlock()
			// if ok {
			// 	// TODO - replace with mutateInPlace code?
			// 	t.UserId = 0
			// 	// serializedMsg, err := serdes.MarshalClientLoginRespMessage(t.UserId, ecs.Id(t.Id))
			// 	serializedMsg, err := clientConn.encoder.Marshal(serdes.ClientLoginResp{t.UserId, ecs.Id(t.Id)})
			// 	if err != nil {
			// 		log.Println("Error Marshalling", err)
			// 		continue
			// 	}
			// 	log.Println("Proxy LoginResp:", t)

			// 	_, err = clientConn.conn.Write(serializedMsg)
			// 	if err != nil {
			// 		log.Println("Error Sending:", err)
			// 		// TODO - User disconnected? Remove from map?
			// 	}
			// } else {
			// 	log.Println("User Disconnected", t.UserId)
			// 	// TODO - Send back to server "hey this person disconnected!"
			// }
		case serdes.ClientLogoutResp:
			log.Println("serdes.ClientLogoutResp")
			// TODO - should I double check that they've been removed from the map?
			// TODO - I should send a "logged out successful" message
		default:
			panic("Unknown message type")
		}
	}

	return nil
}
