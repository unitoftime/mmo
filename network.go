package mmo

import (
	"log"
	"net"
	"sync"

	"go.nanomsg.org/mangos/v3"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/mmo/serdes"
)
type Websocket struct {
	net.Conn
}

func ClientSendUpdate(world *ecs.World, conn net.Conn) {
	ecs.Map2(world, func(id ecs.Id, _ *ClientOwned, input *physics.Input) {
		update := serdes.WorldUpdate{
			WorldData: map[ecs.Id][]ecs.Component{
				id: []ecs.Component{ecs.C(*input)},
			},
		}
		log.Println("ClientSendUpdate:", update)
		serializedInput, err := serdes.MarshalWorldUpdateMessage(update)
		if err != nil {
			log.Println("Flatbuffers, Failed to serialize", err)
		}

		_, err = conn.Write(serializedInput)
		if err != nil {
			log.Println("Error Sending:", err)
			return
		}
	})
}

func ClientReceive(world *ecs.World, conn net.Conn, networkChannel chan serdes.WorldUpdate) {
	const MaxMsgSize int = 4 * 1024

	msg := make([]byte, MaxMsgSize)
	for {
		n, err := conn.Read(msg)

		if err != nil {
			log.Println("Read Error:", err)
			return
		}
		if n <= 0 { continue }

		fbMessage, err := serdes.UnmarshalMessage(msg)
		if err != nil {
			log.Println("Failed to unmarshal:", err)
			continue
		}

		switch t := fbMessage.(type) {
		case serdes.WorldUpdate:
			// log.Println(t)
			networkChannel <- t
		case serdes.ClientLoginResp:
			log.Println("serdes.ClientLoginResp", t)
			// ecs.Write(engine, ecs.Id(t.Id), ClientOwned{})
			// ecs.Write(engine, ecs.Id(t.Id), Body{})
			ecs.Write(world, ecs.Id(t.Id), ecs.C(ClientOwned{}), ecs.C(Body{}))
		default:
			panic("Unknown message type")
		}
	}
}

func ServerSendUpdate(world *ecs.World, sock mangos.Socket, deleteList *DeleteList) {
	deleteList.mu.Lock()
	// TODO - Optimization opportunity: You could have a front-buffer and a back-buffer then toggle which one is the write buffer and which is the read buffer. Then you don't have to copy.
	dListCopy := make([]ecs.Id, len(deleteList.list))
	copy(dListCopy, deleteList.list)

	deleteList.list = deleteList.list[:0]
	deleteList.mu.Unlock()

	update := serdes.WorldUpdate{
		UserId: 0,
		WorldData: make(map[ecs.Id][]ecs.Component),
		Delete: dListCopy,
	}

	{
		ecs.Map2(world, func(id ecs.Id, transform *physics.Transform, body *Body) {
			compList := []ecs.Component{
				ecs.C(*transform),
				ecs.C(*body),
			}
			update.WorldData[id] = compList
		})
	}

	{
		ecs.Map(world, func(id ecs.Id, user *User) {
			update.UserId = user.Id
			// log.Println("ServerSendUpdate WorldUpdate:", update)

			serializedUpdate, err := serdes.MarshalWorldUpdateMessage(update)
			if err != nil {
				log.Println("Error Marshalling", err)
				return
			}

			err = sock.Send(serializedUpdate)
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



func ServeProxyConnection(sock mangos.Socket, world *ecs.World, networkChannel chan serdes.WorldUpdate, deleteList *DeleteList) {
	log.Println("Server: ServeProxyConnection")
	loginMap := make(map[uint64]ecs.Id)

	// Read data
	for {
		msg, err := sock.Recv()
		if err != nil {
			log.Println("Read Error:", err)
		}

		fbMessage, err := serdes.UnmarshalMessage(msg)
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
				ecs.C(Body{}),
				ecs.C(SpawnPoint()),
			)
			// log.Println("Logging in player:", id)

			loginMap[t.UserId] = id

			loginResp := serdes.MarshalClientLoginRespMessage(t.UserId, id)
			err := sock.Send(loginResp)
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

			logoutResp := serdes.MarshalClientLogoutRespMessage(t.UserId, id)
			err := sock.Send(logoutResp)
			if err != nil {
				log.Println("Failed to send logout response")
			}
		default:
			panic("Unknown message type")
		}
	}
}

