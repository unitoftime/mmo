package mmo

import (
	"log"
	"net"
	"encoding/json"

	"go.nanomsg.org/mangos/v3"

	"github.com/jstewart7/mmo/engine/ecs"
	"github.com/jstewart7/mmo/engine/physics"
)

type Websocket struct {
	net.Conn
}
func (t *Websocket) ComponentSet(val interface{}) { *t = val.(Websocket) }

func ClientSendUpdate(engine *ecs.Engine, conn net.Conn) {
	ecs.Each(engine, physics.Input{}, func(id ecs.Id, a interface{}) {
		input := a.(physics.Input)

		serializedInput, err := json.Marshal(input)
		if err != nil {
			log.Println("Failed to serialize", input)
			return
		}

		_, err = conn.Write(serializedInput)
		if err != nil {
			log.Println("Error Sending:", err)
			return
		}
	})
}

func ClientReceive(engine *ecs.Engine, conn net.Conn, networkChannel chan ChannelUpdate) {
	const MaxMsgSize int = 4 * 1024

	msg := make([]byte, MaxMsgSize)
	for {
		n, err := conn.Read(msg)

		if err != nil {
			log.Println("Read Error:", err)
			return
		}
		if n <= 0 { continue }

		serverUpdate := ServerUpdate{}
		err = json.Unmarshal(msg[:n], &serverUpdate)
		if err != nil {
			log.Println("Message didn't match ServerUpdate struct:", msg[:n])
			continue
		}

		// Let the player know that they own this entity
		// TODO - should this go into some other state? ie not a tag
		networkChannel <- ChannelUpdate{
			Id: serverUpdate.PlayerId,
			Component: ClientOwned{},
		}

		for i := range serverUpdate.Transforms {
			id := serverUpdate.Transforms[i].Id
			transform := serverUpdate.Transforms[i].Transform
			networkChannel <- ChannelUpdate{
				Id: id,
				Component: transform,
			}

			// TODO - eventually server will have authority on this
			body := Body{}
			networkChannel <- ChannelUpdate{
				Id: id,
				Component: body,
			}
		}
	}
}

func ServerSendUpdate(engine *ecs.Engine, sock mangos.Socket) {
	transformList := make([]TransformUpdate, 0)

	ecs.Each(engine, physics.Transform{}, func(id ecs.Id, a interface{}) {
		transform := a.(physics.Transform)

		transformList = append(transformList, TransformUpdate{
			Id: id,
			Transform: transform,
		})
	})


	ecs.Each(engine, User{}, func(id ecs.Id, a interface{}) {
		user := a.(User)

		serverUpdate := ServerToProxyMessage{
			Type: "physics",
			Username: user.Name,
			Update: ServerUpdate{
				PlayerId: id,
				Transforms: transformList,
			},
		}

		log.Println(serverUpdate)

		serializedUpdate, err := json.Marshal(serverUpdate)
		if err != nil {
			log.Println("Failed to serialize", serializedUpdate)
			return
		}

		log.Println(string(serializedUpdate))

		err = sock.Send(serializedUpdate)
		if err != nil {
			log.Println("Error Sending:", err)
			return
		}
	})
}

// TODO - find a better place for this
type ProxyToServerMessage struct {
	Type string
	Username string
	Input physics.Input
}

type ServerToProxyMessage struct {
	Type string
	Username string
	Update ServerUpdate
}

type ServerUpdate struct {
	PlayerId ecs.Id
	Transforms []TransformUpdate
}

type TransformUpdate struct {
	Id ecs.Id
	Transform physics.Transform
}
