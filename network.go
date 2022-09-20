package mmo

import (
	"fmt"
	"log"
	"errors"
	"sync"
	"math/rand"
	"time"

	"go.nanomsg.org/mangos/v3"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/mmo/serdes"
	"github.com/unitoftime/mmo/game"
)

// Continually attempts to reconnect to the proxy if disconnected. If connected, receives data and sends over the networkChannel
func ReconnectLoop(world *ecs.World, c *ClientConn) {
	for {
		if c.Closed.Load() { break } // Exit if the ClientConn has been closed

		err := c.Dial()
		if err != nil {
			log.Println("Client Websocket Dial Failed:", err)
			time.Sleep(2 * time.Second) // TODO - reconfigure this before launch. Probably want some random value so everyone isn't reconnecting simultaneously
			continue
		}

		// Start the handler
		err = ClientReceive(world, c, c.updateChan)
		if err != nil {
			log.Println("ClientReceive Exited:", err)

			// TODO - Is this a good idea?
			// Try to close the connection one last time
			c.conn.Close()

			// Set connected to false, because we just closed it
			c.Connected.Store(false)
		}
		log.Println("Looping!")
	}

	// Final attempt to cleanup the connection
	c.Connected.Store(false)
	c.conn.Close()
	log.Println("Exiting ClientConn.ReconnectLoop")
}

type ServerConn struct {
	Encoder *serdes.Serdes
	Sock mangos.Socket
}

func ClientSendUpdate(world *ecs.World, clientConn *ClientConn) {
	// if clientConn is closed for some reason, then we won't be able to send
	// TODO - Is this fast enough?
	connected := clientConn.Connected.Load()
	if !connected { return } // Exit early because we are not connected

	ecs.Map2(world, func(id ecs.Id, _ *ClientOwned, input *physics.Input) {
		update := serdes.WorldUpdate{
			WorldData: map[ecs.Id][]ecs.Component{
				id: []ecs.Component{ecs.C(*input)},
			},
		}
		log.Println("ClientSendUpdate:", update)

		err := clientConn.Send(update)
		if err != nil {
			log.Println(err)
		}
	})
}

func ClientReceive(world *ecs.World, clientConn *ClientConn, networkChannel chan serdes.WorldUpdate) error {
	for {
		msg, err := clientConn.Recv()
		if errors.Is(err, ErrNetwork) {
			// Handle errors where we should stop (ie connection closed or something)
			fmt.Println(err)
			return err
		} else if errors.Is(err, ErrSerdes) {
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
			ecs.Write(world, ecs.Id(t.Id), ecs.C(ClientOwned{}), ecs.C(game.Body{}))
		default:
			panic("Unknown message type")
		}
	}

	return nil
}

func ServerSendUpdate(world *ecs.World, serverConn ServerConn, deleteList *DeleteList) {
	deleteList.mu.Lock()
	// TODO - Optimization opportunity: You could have a front-buffer and a back-buffer then toggle which one is the write buffer and which is the read buffer. Then you don't have to copy.
	dListCopy := make([]ecs.Id, len(deleteList.list))
	copy(dListCopy, deleteList.list)

	deleteList.list = deleteList.list[:0]
	deleteList.mu.Unlock()

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

	// Send world update to al users
	{
		ecs.Map(world, func(id ecs.Id, user *User) {
			update.UserId = user.Id // Specify the user we want to send the update to
			// log.Println("ServerSendUpdate WorldUpdate:", update)

			// serializedUpdate, err := serdes.MarshalWorldUpdateMessage(update)
			serializedUpdate, err := serverConn.Encoder.Marshal(update)
			if err != nil {
				log.Println("Error Marshalling", err)
				return
			}

			log.Println("ServerSendUpdate:", len(serializedUpdate))
			err = serverConn.Sock.Send(serializedUpdate)
			if err != nil {
				log.Println("Error Sending:", err)
				return
			}
		})
	}
}

type DeleteList struct {
	mu sync.RWMutex
	list []ecs.Id
}
func NewDeleteList() *DeleteList {
	return &DeleteList{
		list: make([]ecs.Id, 0),
	}
}

func ServeProxyConnection(serverConn ServerConn, world *ecs.World, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList) {
	log.Println("Server: ServeProxyConnection")
	loginMap := make(map[uint64]ecs.Id)

	// Read data
	for {
		msg, err := serverConn.Sock.Recv()
		if err != nil {
			log.Println("Read Error:", err)
		}

		log.Println(len(msg))
		// TODO - Do mangos sockets automatically slice the msg buffer? I guess they do?
		fbMessage, err := serverConn.Encoder.Unmarshal(msg)
		if err != nil {
			log.Println("Failed to unmarshal:", err)
			continue
		}

		// Interpret different messages
		switch t := fbMessage.(type) {
		case serdes.WorldUpdate:
			id := loginMap[t.UserId]
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
			}),
				ecs.C(physics.Input{}),
				ecs.C(game.Body{uint32(rand.Intn(game.NumBodyTypes))}),
				ecs.C(SpawnPoint()),
			)
			// log.Println("Logging in player:", id)

			loginMap[t.UserId] = id

			// loginResp := serdes.MarshalClientLoginRespMessage(t.UserId, id)
			loginResp, err := serverConn.Encoder.Marshal(serdes.ClientLoginResp{t.UserId, id})
			if err != nil {
				log.Println("Failed to send Marshal")
			}
			log.Println("LoginResp:", len(loginResp))
			err = serverConn.Sock.Send(loginResp)
			if err != nil {
				log.Println("Failed to send login response")
			}
		case serdes.ClientLogout:
			log.Println("Server: serdes.ClientLogout")
			id := loginMap[t.UserId]
			ecs.Delete(world, id)

			delete(loginMap, t.UserId)

			deleteList.mu.Lock()
			deleteList.list = append(deleteList.list, id)
			deleteList.mu.Unlock()

			// logoutResp := serdes.MarshalClientLogoutRespMessage(t.UserId, id)
			logoutResp, err := serverConn.Encoder.Marshal(serdes.ClientLogoutResp{t.UserId, id})
			if err != nil {
				log.Println("Failed to send Marshal")
			}
			log.Println("logoutResp:", len(logoutResp))

			err = serverConn.Sock.Send(logoutResp)
			if err != nil {
				log.Println("Failed to send logout response")
			}
		default:
			panic("Unknown message type")
		}
	}
}

