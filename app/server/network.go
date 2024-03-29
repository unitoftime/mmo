package server

import (
	"fmt"
	"time"
	"errors"
	"sync"
	"math"
	"math/rand"

	"github.com/rs/zerolog/log"

	"github.com/unitoftime/flow/net"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/phy2"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/mmo/stat"
	// "github.com/unitoftime/mmo/game"
)

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

var everyOther int

// var lastTime time.Time
// var lastTime4 time.Time

var AvgWorldUpdateTime time.Duration

var lastWorldUpdate time.Time
// var worldUpdateTimes *mmo.RingBuffer
// func init() {
// 	lastWorldUpdate = time.Now()
// 	bufLen := 100
// 	worldUpdateTimes = mmo.NewRingBuffer(bufLen)
// 	for i := 0; i < bufLen; i++ {
// 		worldUpdateTimes.Add(4 * mmo.FixedTimeStep) // TODO! - hardcoded
// 	}
// }

// This calculates the update to send to all players, finds the proxy associated with them, and sends that update over the wire
func ServerSendUpdate(world *ecs.World, server *Server, deleteList *DeleteList) {
	// log.Print("ServerSendUpdate-LastTime: ", time.Since(lastTime))
	// lastTime = time.Now()
	everyOther = (everyOther + 1) % mmo.NetworkTickDivider
	if everyOther != 0 {
		return // skip
	}
	// log.Print("ServerSendUpdate-LastTime4: ", time.Since(lastTime4))
	// lastTime4 = time.Now()


	dListCopy := deleteList.CopyAndClear()

	// Just delete everything that is gone
	for _, id := range dListCopy {
		ecs.Delete(world, id)
	}

	// Build the world update
	update := serdes.WorldUpdate{
		Tick: server.tick,
		UserId: 0,
		WorldData: make(map[ecs.Id][]ecs.Component),
		Delete: dListCopy,
	}

	//Increment server tick
	server.tick = (server.tick + 1) % math.MaxUint16

	// TODO - [optional ecs feature] speech should be optional!!!!
	// TODO - When you do SOI code, and generate messages on a per player basis. You should also not include the speech bubble that the player just sent.
	// Add relevant data to the world update
	{
		ecs.Map4(world, func(id ecs.Id, pos *phy2.Pos, body *mmo.Body, speech *mmo.Speech, input *mmo.Input) {
			compList := []ecs.Component{
				ecs.C(*pos),
				ecs.C(*body),
				ecs.C(*input),
			}

			if speech.HandleSent() {
				compList = append(compList, ecs.C(*speech))
			}
			update.WorldData[id] = compList
		})
	}

	// Send world update to all users
	{
		ecs.Map2(world, func(id ecs.Id, user *User, clientTick *ClientTick) {
			update.UserId = user.Id // Specify the user we want to send the update to
			// log.Println("ServerSendUpdate WorldUpdate:", update)

			proxy, ok := server.GetProxy(user.ProxyId)
			if !ok {
				log.Print("Missing Proxy for user!")
				// This means that the proxy was disconnected
				deleteList.Append(id) // This deletes the user (ie they logged out)
				return
			}

			// Set the player's update tick so they can synchronize
			update.PlayerTick = clientTick.Tick

			// log.Printf("SendUpdate", update)
			err := proxy.Send(update)
			if err != nil {
				log.Warn().Err(err).Msg("ServerSendUpdate")
				return
			}
		})
	}

	// {
	// 	worldUpdateTimes.Add(time.Since(lastWorldUpdate))
	// 	lastWorldUpdate = time.Now()
	// 	buf := worldUpdateTimes.Buffer()
	// 	AvgWorldUpdateTime = 0
	// 	for i := range buf {
	// 		AvgWorldUpdateTime += buf[i]
	// 	}
	// 	AvgWorldUpdateTime = AvgWorldUpdateTime / time.Duration(len(buf))
	// 	log.Print("Server-AvgWorldUpdateTime: ", AvgWorldUpdateTime)
	// }
}

func ServeProxyConnection(serverConn *ServerConn, world *ecs.World, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList) error {
	log.Print("Server: ServeProxyConnection")

	// Read data
	for {
		msg, err := serverConn.Recv()
		if errors.Is(err, net.ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			log.Warn().Err(err).Msg("ServeProxyConnection NetworkErr")
			return err
		} else if errors.Is(err, net.ErrSerdes) {
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

			compSlice := make([]ecs.Component, 0)
			// TODO - these should be in a loop. can't guarantee each component slot
			inputBox, ok := componentList[0].(ecs.CompBox[mmo.Input])
			if !ok { continue }
			input := inputBox.Get()
			compSlice = append(compSlice, ecs.C(input))

			if len(componentList) > 1 {
				speechBox, ok := componentList[1].(ecs.CompBox[mmo.Speech])
				if !ok { continue }
				speech := speechBox.Get()
				compSlice = append(compSlice, ecs.C(speech))
			}

			// We just send this field back to the player, we don't use it internally. This is for them to syncrhonize their client prediction.
			compSlice = append(compSlice, ecs.C(ClientTick{
				Tick: t.PlayerTick,
			}))

			trustedUpdate := serdes.WorldUpdate{
				WorldData: map[ecs.Id][]ecs.Component{
					id: compSlice,
				},
				// Messages: t.Messages,
			}
			// log.Print("TrustedUpdate:", trustedUpdate)

			networkChannel <- trustedUpdate

		case serdes.ClientLogin:
			log.Print("Server: serdes.ClientLogin")
			// Login player
			// TODO! - not threadsafe
			id := world.NewId()

			// TODO - hardcoded here and in client.go - Centralize character creation
			collider := phy2.NewCircleCollider(6)
			collider.Layer = mmo.BodyLayer
			collider.HitLayer = mmo.BodyLayer
			trustedLogin := serdes.WorldUpdate{
				WorldData: map[ecs.Id][]ecs.Component{
					id: []ecs.Component{
						ecs.C(User{
							Id: t.UserId,
							ProxyId: serverConn.proxyId,
						}),
						ecs.C(mmo.Input{}),
						ecs.C(mmo.Body{uint32(rand.Intn(mmo.NumBodyTypes))}),
						ecs.C(mmo.Speech{}),
						ecs.C(mmo.SpawnPoint()),
						ecs.C(collider),
						ecs.C(phy2.NewColliderCache()),
					},
				},
			}
			networkChannel <- trustedLogin

			serverConn.LoginUser(t.UserId, id)

			resp := serdes.ClientLoginResp{t.UserId, id}
			err := serverConn.Send(resp)
			if err != nil {
				log.Warn().Err(err).Msg(fmt.Sprintf("Failed to send: %v", resp))
			}
		case serdes.ClientLogout:
			log.Printf("serdes.ClientLogout: %d", t.UserId)
			id, ok := serverConn.GetUser(t.UserId)
			if !ok {
				// Skip: User already logged out
				log.Printf("User already logged out: %d", t.UserId)
				continue
			}
			trustedLogout := serdes.WorldUpdate{
				UserId: t.UserId,
				Delete: []ecs.Id{id},
			}
			networkChannel <- trustedLogout

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
	sock *net.Socket

	mu sync.RWMutex
	proxyId uint64
	loginMap map[uint64]ecs.Id
}

func (c *ServerConn) Send(msg any) error {
	return c.sock.Send(msg)
}

func (c *ServerConn) Recv() (any, error) {
	return c.sock.Recv()
}

func (c *ServerConn) LoginUser(userId uint64, ecsId ecs.Id) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loginMap[userId] = ecsId
}

func (c *ServerConn) LogoutUser(userId uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.loginMap, userId)
}

func (c *ServerConn) GetUser(userId uint64) (ecs.Id, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ret, ok := c.loginMap[userId]
	return ret, ok
}

// TODO - add more stats
func (c *ServerConn) GetStats() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ret := len(c.loginMap)
	return ret
}

type Server struct {
	listener net.Listener
	handler func(*ServerConn) error

	tick uint16

 	connectionsMut sync.RWMutex // Sync for connections map
	connections map[uint64]*ServerConn // A map of proxyIds to Proxy connections
}

func NewServer(listener net.Listener, handler func(*ServerConn) error) *Server {
	server := Server{
		listener: listener,
		connections: make(map[uint64]*ServerConn),
		handler: handler,
	}
	return &server
}


func (s *Server) Start() {
	// Debug: Print out server stats
	go func() {
		for {
			time.Sleep(10 * time.Second)
			s.connectionsMut.RLock()
			for proxyId, proxyConn := range s.connections {
				numActive := proxyConn.GetStats()
				log.Print(fmt.Sprintf("Proxy %d - %d active users", proxyId, numActive))
			}
			s.connectionsMut.RUnlock()
		}
	}()

	counter := uint64(0)
	for {
		// Wait for a connection.
		sock, err := s.listener.Accept()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to accept connection")
			continue
		}

		proxyId := counter
		serverConn := &ServerConn{
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

func (s *Server) GetProxy(proxyId uint64) (*ServerConn, bool) {
 	s.connectionsMut.RLock()
	defer s.connectionsMut.RUnlock()

	c, ok := s.connections[proxyId]
	return c, ok
}

func (s *Server) AddProxy(proxyId uint64, conn *ServerConn) {
	s.connectionsMut.Lock()
	defer s.connectionsMut.Unlock()
	s.connections[proxyId] = conn
}

func (s *Server) RemoveProxy(proxyId uint64) {
	s.connectionsMut.Lock()
	defer s.connectionsMut.Unlock()
	delete(s.connections, proxyId)
}


//--------------------------------------------------------------------------------
// - Handle Capturing data from network
//--------------------------------------------------------------------------------

// type LastUpdate struct {
// 	Time time.Time
// }

// type EcsUpdate struct {
// 	WorldData map[ecs.Id][]ecs.Component
// 	Delete []ecs.Id
// }

// TODO - this kindof represents a greater pattern of trying to apply commands to the world in a threadsafe manner. Maybe integrate this into the ECS library: https://docs.rs/bevy/0.4.0/bevy/ecs/trait.Command.html
func CreatePollNetworkSystem(world *ecs.World, networkChannel chan serdes.WorldUpdate) ecs.System {
	sys := ecs.System{"PollNetworkChannel", func(dt time.Duration) {

	MainLoop:
		for {
			select {
			case update := <-networkChannel:
				for id, compList := range update.WorldData {
					// compList = append(compList, ecs.C(LastUpdate{time.Now()}))
					ecs.Write(world, id, compList...)
				}

				// Delete all the entities in the deleteList
				if update.Delete != nil {
					for _, id := range update.Delete {
						ecs.Delete(world, id)
					}
				}

			default:
				break MainLoop
			}
		}
	}}

	return sys
}

