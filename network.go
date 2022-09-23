package mmo

import (
	"fmt"
	"log"
	"errors"
	"sync"
	"math/rand"
	"net"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/mmo/mnet"
	"github.com/unitoftime/mmo/game"
)

func ClientSendUpdate(world *ecs.World, clientConn *mnet.Socket, playerId ecs.Id) {
	// if clientConn is closed for some reason, then we won't be able to send
	// TODO - Is this fast enough?
	connected := clientConn.Connected.Load()
	if !connected { return } // Exit early because we are not connected

	input, ok := ecs.Read[physics.Input](world, playerId)
	if !ok { return } // If we can't find the players input just exit early

	update := serdes.WorldUpdate{
		WorldData: map[ecs.Id][]ecs.Component{
			playerId: []ecs.Component{ecs.C(input)},
		},
	}
	log.Println("ClientSendUpdate:", update)

	err := clientConn.Send(update)
	if err != nil {
		log.Println(err)
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

func ClientReceive(world *ecs.World, sock *mnet.Socket, playerId *ecs.Id, networkChannel chan serdes.WorldUpdate) error {
	for {
		msg, err := sock.Recv()
		if errors.Is(err, mnet.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			fmt.Println(err)
			return err
		} else if errors.Is(err, mnet.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			fmt.Println(err)
			continue
		}
		if msg == nil { continue }

		switch t := msg.(type) {
		case serdes.WorldUpdate:
			// log.Println(t)
			networkChannel <- t
		case serdes.ClientLoginResp:
			log.Println("serdes.ClientLoginResp", t)
			// ecs.Write(engine, ecs.Id(t.Id), ClientOwned{})
			// ecs.Write(engine, ecs.Id(t.Id), Body{})
			// TODO - is this a hack? Should I be using the networkChannel?
			// ecs.Write(world, ecs.Id(t.Id), ecs.C(ClientOwned{}), ecs.C(game.Body{}))
			ecs.Write(world, ecs.Id(t.Id), ecs.C(game.Body{}))
			*playerId = ecs.Id(t.Id)
		default:
			panic("Unknown message type")
		}
	}

	return nil
}

// --------------------------------------------------------------------------------
// - Server
// --------------------------------------------------------------------------------
// This is the connection from the server to a proxy
// type ServerConn struct {
// 	sock *mnet.Socket
// 	// encoder *serdes.Serdes
// 	// conn mnet.Conn

// 	proxyId uint64
// 	loginMap map[uint64]ecs.Id
// }

// func (c *ServerConn) Send(msg any) error {
// 	return c.sock.Send(msg)
// }

// func (c *ServerConn) Recv() (any, error) {
// 	return c.sock.Recv()
// }

// func (c *ServerConn) Send(msg any) error {
// 	ser, err := c.encoder.Marshal(msg)
// 	if err != nil {
// 		log.Println("Failed to serialize", err)
// 	}

// 	// log.Println("ServerSendUpdate:", len(ser))
// 	_, err = c.conn.Write(ser)
// 	if err != nil {
// 		log.Println("Error Sending:", err)
// 		err = fmt.Errorf("%w: %s", ErrNetwork, err)
// 		return err
// 	}
// 	return nil
// }

// func (c *ServerConn) Recv() (any, error) {
// 	// TODO optimize this buffer creation
// 	const MaxMsgSize int = 4 * 1024
// 	dat := make([]byte, MaxMsgSize)

// 	n, err := c.conn.Read(dat)
// 	if err != nil {
// 		log.Println("Read Error:", err)
// 		err = fmt.Errorf("%w: %s", ErrNetwork, err)
// 		return nil, err
// 	}
// 	if n <= 0 { return nil, nil } // There was no message, and no error (likely a keepalive)

// 	log.Println("Read bytes", n)

// 	// Note: slice off based on how many bytes we read
// 	msg, err := c.encoder.Unmarshal(dat[:n])
// 	if err != nil {
// 		log.Println("Failed to unmarshal:", err)
// 		err = fmt.Errorf("%w: %s", ErrSerdes, err)
// 		return nil, err
// 	}
// 	return msg, nil
// }

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
	// Debug: Print out the number of active/logged in users
	for proxyId, proxyConn := range server.connections {
		log.Println(fmt.Sprintf("Proxy %d - %d active users", proxyId, len(proxyConn.loginMap)))
	}

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
				log.Println("Missing Proxy for user!")
				// This means that the proxy was disconnected
				deleteList.Append(id) // This deletes the user (ie they logged out)
				return
			}

			err := proxy.Send(update)
			if err != nil {
				log.Println("ServerSendUpdate", err)
				return
			}
		})
	}
}

func ServeProxyConnection(serverConn ServerConn, world *ecs.World, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList) error {
	log.Println("Server: ServeProxyConnection")

	// Read data
	for {
		msg, err := serverConn.Recv()
		if errors.Is(err, mnet.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			// TODO - when this triggers, I need to have the eventual code path to: 1. logout all users who were on this proxy. 2. make sure the socket is closed (ie any cleanup)
			fmt.Println(err)
			return err
		} else if errors.Is(err, mnet.ErrSerdes) {
			// Handle errors where we should continue (ie serialization)
			fmt.Println(err)
			continue
		}
		if msg == nil { continue }

		// Interpret different messages
		switch t := msg.(type) {
		case serdes.WorldUpdate:
			id := serverConn.loginMap[t.UserId]
			// TODO - requires client to put their input on spot 0
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
			log.Println("TrustedUpdate:", trustedUpdate)

			networkChannel <- trustedUpdate

		case serdes.ClientLogin:
			log.Println("Server: serdes.ClientLogin")
			// Login player
			// TODO - put into a function
			// TODO - not thread safe! Concurrent map access
			// TODO - Refactor networking layer to have an RPC functionality
			id := world.NewId()
			ecs.Write(world, id, ecs.C(User{
				Id: t.UserId,
				ProxyId: serverConn.proxyId,
			}),
				ecs.C(physics.Input{}),
				ecs.C(game.Body{uint32(rand.Intn(game.NumBodyTypes))}),
				ecs.C(SpawnPoint()),
			)
			// log.Println("Logging in player:", id)

			serverConn.loginMap[t.UserId] = id

			resp := serdes.ClientLoginResp{t.UserId, id}
			err := serverConn.Send(resp)
			if err != nil {
				log.Println("Failed to send", resp)
			}
		case serdes.ClientLogout:
			log.Println("Server: serdes.ClientLogout")
			id := serverConn.loginMap[t.UserId]
			ecs.Delete(world, id)

			delete(serverConn.loginMap, t.UserId)

			deleteList.Append(id)

			resp := serdes.ClientLogoutResp{t.UserId, id}
			err := serverConn.Send(resp)
			if err != nil {
				log.Println("Failed to send", resp)
			}
		default:
			panic("Unknown message type")
		}
	}
}


//--------------------------------------------------------------------------------
type ServerConn struct {
	sock *mnet.Socket
	// encoder *serdes.Serdes
	// conn mnet.Conn

	proxyId uint64
	loginMap map[uint64]ecs.Id
}

func (c *ServerConn) Send(msg any) error {
	return c.sock.Send(msg)
}

func (c *ServerConn) Recv() (any, error) {
	return c.sock.Recv()
}

type Server struct {
	listener net.Listener
	connections map[uint64]ServerConn // A map of proxyIds to Proxy connections
	handler func(ServerConn) error
}

// TODO - use mnet.URL?
func NewServer(url string, handler func(ServerConn) error) *Server {
	listener, err := net.Listen("tcp", url)
	if err != nil {
		panic(err)
	}

	server := Server{
		listener: listener,
		connections: make(map[uint64]ServerConn),
		handler: handler,
	}
	return &server
}


func (s *Server) Start() {
	counter := uint64(0)
	for {
		// Wait for a connection.
		conn, err := s.listener.Accept()
		if err != nil {
			log.Println("Failed to accept connection", err)
		}

		// TODO - is this bad? This socket should never get dialed
		sock := mnet.NewConnectedSocket(conn)

		proxyId := counter
		serverConn := ServerConn{
			sock: sock,
			// encoder: serdes.New(),
			// conn: conn,
			proxyId: proxyId,
			loginMap: make(map[uint64]ecs.Id),
		}

		s.AddProxy(proxyId, serverConn)

		counter++
		go func() {
			err := s.handler(serverConn)
			if err != nil {
				log.Println(err)
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

