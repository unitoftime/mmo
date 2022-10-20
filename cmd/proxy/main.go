package main

import (
	"fmt"
	"os"
	"os/signal"
	"net/http"
	"crypto/tls"
	"sync"
	"net"
	"time"
	"context"
	"errors"
	"embed"
	"io/ioutil"

	"gopkg.in/yaml.v3"

	"nhooyr.io/websocket"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/unitoftime/mmo/stat"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/mmo/mnet"
	"github.com/unitoftime/mmo/game"
	"github.com/unitoftime/ecs"
)

//go:embed config.yaml
var fs embed.FS

type Config struct {
	Server string
	KeyFile string
	CertFile string
	Test bool
}
func LoadConfig() (*Config, error) {
	file, err := fs.Open("config.yaml")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	yamlData, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = yaml.Unmarshal(yamlData, config)
	return config, err
}


func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	config, err := LoadConfig()
	if err != nil { panic(err) }

	// url := "tcp://127.0.0.1:9000"
	url := config.Server

	room := NewRoom()

	sock, err := mnet.NewSocket(url, config.Test)
	if err != nil {
		panic(err)
	}

	go mnet.ReconnectLoop(sock, func(sock *mnet.Socket) error {
		// After we reconnect the proxy to the server, we want to log all the players into the server who were waiting.
		room.mu.RLock()
		for userId := range room.Map {
			log.Debug().Uint64(stat.UserId, userId).Msg("Reconnect - Sending Login Message for")

			loginMsg := serdes.ClientLogin{userId}
			err := sock.Send(loginMsg)
			if err != nil {
				log.Error().Err(err).Uint64(stat.UserId, userId).Msg("Failed to send login message")
			}
		}
		room.mu.RUnlock()

		return room.HandleGameUpdates(sock)
	})
	// go mmo.ReconnectLoop(world, clientConn, &playerId, networkChannel)

	// HTTP Version
	// listener, err := net.Listen("tcp", ":443")

	// HTTPS Version
	certPem, err := os.ReadFile(config.CertFile)
	if err != nil {
		panic(err)
	}
	keyPem, err := os.ReadFile(config.KeyFile)
	if err != nil {
		panic(err)
	}
	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		panic(err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	hostname := ":443"
	if config.Test {
		hostname = "localhost:7777"
	}
	listener, err := tls.Listen("tcp", hostname, tlsConfig)
	if err != nil {
		panic(err)
	}

	// go room.HandleGameUpdates(serverConn)

	s := &http.Server{
		TLSConfig: tlsConfig,
		Handler: websocketServer{
			serverConn: sock,
			room: room,
		},
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Print("Starting Proxy", listener.Addr())

	errc := make(chan error, 1)
	go func() {
		errc <- s.Serve(listener)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	select{
	case err := <-errc:
		log.Error().Err(err).Msg("Failed to serve")
	case sig := <-sigs:
		log.Print(fmt.Sprintf("Terminating: %v", sig))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()

	err = s.Shutdown(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to shut down server")
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
		OriginPatterns: []string{"localhost:8081", "mmo.unit.dev", "unit.dev", "www.unit.dev"},
	})
	if err != nil {
		log.Error().Err(err).Msg("Error Accepting Websocket")
		return
	}

	ctx := context.Background()

	conn := websocket.NetConn(ctx, c, websocket.MessageBinary)

	go ServeNetConn(conn, s.serverConn, s.room)
}

// This is just to make sure different users get different login ids
var userIdCounter uint64

// Handles the websocket connection to a specific client in the room
func ServeNetConn(conn net.Conn, serverConn *mnet.Socket, room *Room) {
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Error().Err(err).Msg("Error closing websocket connection")
		}
	}()

	timeoutSeconds := 60 * time.Second
	timeout := make(chan uint8, 1)
	const StopTimeout uint8 = 0
	const ContTimeout uint8 = 1

	// Login player
	room.mu.Lock()
	// TODO - Eventually This id should come from the login request which probably has a JWT which encodes the data. You probably don't need that in a lock
	userId := userIdCounter
	userIdCounter++
	_, ok := room.Map[userId]
	if ok {
		log.Print("Duplicate Login Detected! Exiting.")
		room.mu.Unlock()
		return
	}

	sock := mnet.NewConnectedSocket(conn)
	room.Map[userId] = ClientConnection{sock}

	room.mu.Unlock()

	// Cleanup room once they leave
	defer func() {
		room.mu.Lock()
		delete(room.Map, userId)
		room.mu.Unlock()
	}()

	// Send login message to server
	log.Debug().Uint64(stat.UserId, userId).Msg("Sending Login Message")
	err := serverConn.Send(serdes.ClientLogin{userId})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to forward login message")
		return
	}

	// Send logout message to server
	defer func() {
		sendUserLogoutToServer(serverConn, userId)
	}()

	// Read data from client and sends to game server
	// TODO - (When I migrate to TCP) TCP doesn't provide framing, so message framing needs to be added
	// TODO - (When I migrate to TCP) TCP will send 0 byte messages to indicate closes, websockets sends them without closing
	go func() {
		for {
			msg, err := sock.Recv()
			if errors.Is(err, mnet.ErrNetwork) {
				timeout <- StopTimeout // Stop timeout because of a read error
				log.Warn().Err(err).Msg("Failed to receive")
				return
			} else if errors.Is(err, mnet.ErrSerdes) {
				// Handle errors where we should continue (ie serialization)
				log.Error().Err(err).Msg("Failed to serialize")
				continue
			}

			// Tick the timeout watcher so we don't timeout!
			timeout <- ContTimeout

			// If the message was empty, just continue to the next one
			if msg == nil { continue }

			switch t := msg.(type) {
			case serdes.WorldUpdate:
				// log.Printf("%v", t.Messages)
				// for i := range t.Messages {
				// 	t.Messages[i].Filter() // Run a chat filter
				// }

				// Filter chat messages
				for _, compSlice := range t.WorldData {
					for i, c := range compSlice {
						switch t := c.(type) {
						case ecs.CompBox[game.Speech]:
							filteredText := game.FilterChat(t.Get().Text)
							log.Print("Chat Speech: ", t.Get().Text)
							log.Print("Chat Filter: ", filteredText)
							compSlice[i] = ecs.C(game.Speech{
								Text: filteredText,
							})
						}
					}
				}

				t.UserId = userId

				err := serverConn.Send(t)
				if err != nil {
					log.Warn().Err(err).Msg("Failed to send")
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
				log.Print("Manually Stopping Timeout Manager")
				break ExitTimeout
			}
		case <-time.After(timeoutSeconds):
			log.Print("User timed out!")
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
		log.Print("User Disconnected", userId)
		return nil
	}

	return &clientConn
}

// Read data from game server and send to client
func (r *Room) HandleGameUpdates(serverConn *mnet.Socket) error {
	for {
		msg, err := serverConn.Recv()
		if errors.Is(err, mnet.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			log.Warn().Err(err).Msg("HandleGameUpdates NetError")
			return err
		} else if errors.Is(err, mnet.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			log.Error().Err(err).Msg("HandleGameUpdates SerdesError")
			continue
		}
		if msg == nil { continue }

		// log.Printf("GameUpdate: %v", msg)

		switch t := msg.(type) {
		case serdes.WorldUpdate:
			clientConn := r.GetClientConn(t.UserId)
			if clientConn == nil {
				// TODO - minor hack: Just remind server that they're disconnected
				sendUserLogoutToServer(serverConn, t.UserId)
				continue
			}

			t.UserId = 0 // Clear userId (clients don't need to know user IDs)
			err := clientConn.sock.Send(t)
			if err != nil {
				log.Warn().Err(err).Msg("Error Sending WorldUpdate to user")
				// TODO - User disconnected? Remove from map? Why is server still sending to them?
			}
		case serdes.ClientLoginResp:
			clientConn := r.GetClientConn(t.UserId)
			if clientConn == nil { continue }

			err := clientConn.sock.Send(serdes.ClientLoginResp{t.UserId, ecs.Id(t.Id)})
			if err != nil {
				log.Warn().Err(err).Msg("Error Sending login response to user")
				// TODO - User disconnected? Remove from map? Why is server still sending to them?
			}

		case serdes.ClientLogoutResp:
			log.Print("Received serdes.ClientLogoutResp")
			// Note: When the proxy's client connection handler function exits, it removes the user from the room.
			// TODO - Do I want to send a message to the user that says "Logout Successful"?
		default:
			log.Error().
				Err(fmt.Errorf("Server Sent unknown message type %T", msg)).
				Msg("HandleGameUpdates")
		}
	}

	return nil
}

func sendUserLogoutToServer(sock *mnet.Socket, userId uint64) {
	err := sock.Send(serdes.ClientLogout{userId})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to send logout message")
		panic(err)
	}
	log.Printf("SendUserLogoutToServer: %d", userId)
}
