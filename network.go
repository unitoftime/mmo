package mmo

import (
	"fmt"
	"time"
	"errors"
	"sync"
	"math/rand"
	"net"

	"github.com/rs/zerolog/log"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/flow/render"
	"github.com/unitoftime/glitch"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/mmo/mnet"
	"github.com/unitoftime/mmo/stat"
	"github.com/unitoftime/mmo/game"
)

func ClientSendUpdate(world *ecs.World, clientConn *mnet.Socket, playerData *PlayerData) {
	playerId := playerData.Id()
	// if clientConn is closed for some reason, then we won't be able to send
	// TODO - With the atomic this fast enough?
	connected := clientConn.Connected.Load()
	if !connected { return } // Exit early because we are not connected

	input, ok := ecs.Read[physics.Input](world, playerId)
	if !ok { return } // If we can't find the players input just exit early

	update := serdes.WorldUpdate{
		WorldData: map[ecs.Id][]ecs.Component{
			playerId: []ecs.Component{ecs.C(input)},
		},
	}
	// log.Print("ClientSendUpdate:", update)

	err := clientConn.Send(update)
	if err != nil {
		log.Warn().Err(err).Msg("ClientSendUpdate")
	}

	// ecs.Map2(world, func(id ecs.Id, _ *ClientOwned, input *physics.Input) {
	// 	update := serdes.WorldUpdate{
	// 		WorldData: map[ecs.Id][]ecs.Component{
	// 			id: []ecs.Component{ecs.C(*input)},
	// 		},
	// 	}
	// 	log.Println("ClientSendUpdate:", update)

	// 	err := clientConn.Send(update)
	// 	if err != nil {
	// 		log.Println(err)
	// 	}
	// })
}

func ClientReceive(sock *mnet.Socket, playerData *PlayerData, networkChannel chan serdes.WorldUpdate) error {
	for {
		msg, err := sock.Recv()
		if errors.Is(err, mnet.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			log.Warn().Err(err).Msg("ClientReceive NetworkErr")
			return err
		} else if errors.Is(err, mnet.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			log.Error().Err(err).Msg("ClientReceive SerdesErr")
			continue
		}
		if msg == nil { continue }

		switch t := msg.(type) {
		case serdes.WorldUpdate:
			networkChannel <- t
		case serdes.ClientLoginResp:
			log.Print("serdes.ClientLoginResp", t)
			// TODO this might be needed in the future if I want to write any data on login resp
			// ecs.Write(world, ecs.Id(t.Id), ecs.C(game.Body{}))
			// networkChannel <- serdes.WorldUpdate{
			// 	UserId: t.UserId,
			// 	WorldData: map[ecs.Id][]ecs.Component{
			// 		ecs.Id(t.Id): []ecs.Component{
			// 			ecs.C(game.Body{}),
			// 		},
			// 	},
			// }

			playerData.SetId(t.Id)

			networkChannel <- serdes.WorldUpdate{
				UserId: t.UserId,
				WorldData: map[ecs.Id][]ecs.Component{
					ecs.Id(t.Id): []ecs.Component{
						ecs.C(physics.Input{}),
						ecs.C(render.Keybinds{
							Up: glitch.KeyW,
							Down: glitch.KeyS,
							Left: glitch.KeyA,
							Right: glitch.KeyD,
						}),
					},
				},
			}

		default:
			log.Error().Msg("Unknown message type")
		}
	}

	return nil
}

// --------------------------------------------------------------------------------
// - Server
// --------------------------------------------------------------------------------
type DeleteList struct {
	mu sync.RWMutex
	list []ecs.Id
}
func NewDeleteList() *DeleteList {
	return &DeleteList{
		list: make([]ecs.Id, 0),
	}
}

func (d *DeleteList) Append(id ecs.Id) {
	d.mu.Lock()
	d.list = append(d.list, id)
	d.mu.Unlock()
}

func (d *DeleteList) CopyAndClear() []ecs.Id {
	d.mu.Lock()
	// TODO - Optimization opportunity: You could have a front-buffer and a back-buffer then toggle which one is the write buffer and which is the read buffer. Then you don't have to copy.
	dListCopy := make([]ecs.Id, len(d.list))
	copy(dListCopy, d.list)

	d.list = d.list[:0]
	d.mu.Unlock()
	return dListCopy
}

// This calculates the update to send to all players, finds the proxy associated with them, and sends that update over the wire
func ServerSendUpdate(world *ecs.World, server *Server, deleteList *DeleteList) {
	dListCopy := deleteList.CopyAndClear()

	// Just delete everything that is gone
	for _, id := range dListCopy {
		ecs.Delete(world, id)
	}

	// Build the world update
	update := serdes.WorldUpdate{
		UserId: 0,
		WorldData: make(map[ecs.Id][]ecs.Component),
		Delete: dListCopy,
	}

	// Add relevant data to the world update
	{
		ecs.Map2(world, func(id ecs.Id, transform *physics.Transform, body *game.Body) {
			compList := []ecs.Component{
				ecs.C(*transform),
				ecs.C(*body),
			}
			update.WorldData[id] = compList
		})
	}

	// Send world update to all users
	{
		ecs.Map(world, func(id ecs.Id, user *User) {
			update.UserId = user.Id // Specify the user we want to send the update to
			// log.Println("ServerSendUpdate WorldUpdate:", update)

			proxy, ok := server.GetProxy(user.ProxyId)
			if !ok {
				log.Print("Missing Proxy for user!")
				// This means that the proxy was disconnected
				deleteList.Append(id) // This deletes the user (ie they logged out)
				return
			}

			err := proxy.Send(update)
			if err != nil {
				log.Warn().Err(err).Msg("ServerSendUpdate")
				return
			}
		})
	}
}

func ServeProxyConnection(serverConn ServerConn, world *ecs.World, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList) error {
	log.Print("Server: ServeProxyConnection")

	// Read data
	for {
		msg, err := serverConn.Recv()
		if errors.Is(err, mnet.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			log.Warn().Err(err).Msg("ServeProxyConnection NetworkErr")
			return err
		} else if errors.Is(err, mnet.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			log.Error().Err(err).Msg("ServeProxyConnection SerdesErr")
			continue
		}
		if msg == nil { continue }

		// Interpret different messages
		switch t := msg.(type) {
		case serdes.WorldUpdate:
			id, ok := serverConn.GetUser(t.UserId)
			if !ok {
				log.Error().Uint64(stat.UserId, t.UserId).
					Msg("Proxy sent update for user that we don't have on the server")
				// Skip: We can't find the user
				continue
			}

			// TODO - requires client to put their input on spot 0. You probably want to change the message serialization type to just send one piece of entity data over.
			componentList := t.WorldData[id]
			if len(componentList) <= 0 { break } // Exit if no content

			inputBox, ok := componentList[0].(ecs.CompBox[physics.Input]) // TODO - should id be replaced with 0?
			if !ok { continue }
			input := inputBox.Get()

			trustedUpdate := serdes.WorldUpdate{
				WorldData: map[ecs.Id][]ecs.Component{
					id: []ecs.Component{ecs.C(input)},
				},
			}
			// log.Print("TrustedUpdate:", trustedUpdate)

			networkChannel <- trustedUpdate

		case serdes.ClientLogin:
			log.Print("Server: serdes.ClientLogin")
			// Login player
			id := world.NewId()
			trustedLogin := serdes.WorldUpdate{
				WorldData: map[ecs.Id][]ecs.Component{
					id: []ecs.Component{
						ecs.C(User{
							Id: t.UserId,
							ProxyId: serverConn.proxyId,
						}),
						ecs.C(physics.Input{}),
						ecs.C(game.Body{uint32(rand.Intn(game.NumBodyTypes))}),
						ecs.C(SpawnPoint()),
					},
				},
			}
			networkChannel <- trustedLogin

			serverConn.LoginUser(t.UserId, id)

			resp := serdes.ClientLoginResp{t.UserId, id}
			err := serverConn.Send(resp)
			if err != nil {
				log.Warn().Err(err).Msg(fmt.Sprintf("Failed to send", resp))
			}
		case serdes.ClientLogout:
			log.Print("Server: serdes.ClientLogout")
			id, ok := serverConn.GetUser(t.UserId)
			if !ok {
				// Skip: User already logged out
				continue
			}
			ecs.Delete(world, id)

			serverConn.LogoutUser(t.UserId)

			deleteList.Append(id)

			resp := serdes.ClientLogoutResp{t.UserId, id}
			err := serverConn.Send(resp)
			if err != nil {
				log.Print("Failed to send", resp)
			}
		default:
			log.Error().Msg("Unknown message type")
		}
	}
}


//--------------------------------------------------------------------------------
type ServerConn struct {
	sock *mnet.Socket

	mu sync.RWMutex
	proxyId uint64
	loginMap map[uint64]ecs.Id // TODO - this isn't currently being synchronized, it is only used from the server handler function currently
}

func (c *ServerConn) Send(msg any) error {
	return c.sock.Send(msg)
}

func (c *ServerConn) Recv() (any, error) {
	return c.sock.Recv()
}

func (c *ServerConn) LoginUser(userId uint64, ecsId ecs.Id) {
	c.mu.Lock()
	c.loginMap[userId] = ecsId
	c.mu.Unlock()
}

func (c *ServerConn) LogoutUser(userId uint64) {
	c.mu.Lock()
	delete(c.loginMap, userId)
	c.mu.Unlock()
}

func (c *ServerConn) GetUser(userId uint64) (ecs.Id, bool) {
	c.mu.RLock()
	ret, ok := c.loginMap[userId]
	c.mu.RUnlock()
	return ret, ok
}

// TODO - add more stats
func (c *ServerConn) GetStats() int {
	c.mu.RLock()
	ret := len(c.loginMap)
	c.mu.RUnlock()
	return ret
}

type Server struct {
	listener net.Listener
	connections map[uint64]ServerConn // A map of proxyIds to Proxy connections
	handler func(ServerConn) error
}

// TODO - use net.URL?
func NewServer(url string, handler func(ServerConn) error) (*Server, error) {
	listener, err := net.Listen("tcp", url)
	if err != nil {
		return nil, err
	}

	server := Server{
		listener: listener,
		connections: make(map[uint64]ServerConn),
		handler: handler,
	}
	return &server, nil
}


func (s *Server) Start() {
	// Debug: Print out server stats
	go func() {
		for {
			time.Sleep(10 * time.Second)
			for proxyId, proxyConn := range s.connections {
				// TODO - race condition here with checking the map length
				log.Print(fmt.Sprintf("Proxy %d - %d active users", proxyId, proxyConn.GetStats()))
			}
		}
	}()

	counter := uint64(0)
	for {
		// Wait for a connection.
		conn, err := s.listener.Accept()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to accept connection")
		}

		// TODO - is this bad? This socket should never get dialed
		sock := mnet.NewConnectedSocket(conn)

		proxyId := counter
		serverConn := ServerConn{
			sock: sock,
			proxyId: proxyId,
			loginMap: make(map[uint64]ecs.Id),
		}

		s.AddProxy(proxyId, serverConn)

		counter++
		go func() {
			err := s.handler(serverConn)
			if err != nil {
				log.Warn().Err(err).Msg("Server Handler finished")
			}

			// Once the handler exits remove the proxy
			s.RemoveProxy(proxyId)
		}()
	}
}

func (s *Server) GetProxy(proxyId uint64) (ServerConn, bool) {
	c, ok := s.connections[proxyId]
	return c, ok
}

func (s *Server) AddProxy(proxyId uint64, conn ServerConn) {
	s.connections[proxyId] = conn
}

func (s *Server) RemoveProxy(proxyId uint64) {
	delete(s.connections, proxyId)
}

