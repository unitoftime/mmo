package mmo

import (
	"log"
	"net"
	"encoding/json"

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

		// log.Println(input)
		// Serialize
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

type ServerUpdate struct {
	PlayerId ecs.Id
	Transforms []TransformUpdate
}

type TransformUpdate struct {
	Id ecs.Id
	Transform physics.Transform
}

func ServerSendUpdate(engine *ecs.Engine) {
	transformList := make([]TransformUpdate, 0)

	ecs.Each(engine, physics.Transform{}, func(id ecs.Id, a interface{}) {
		transform := a.(physics.Transform)

		transformList = append(transformList, TransformUpdate{
			Id: id,
			Transform: transform,
		})
	})

	ecs.Each(engine, Websocket{}, func(id ecs.Id, a interface{}) {
		websocket := a.(Websocket)

		serverUpdate := ServerUpdate{
			PlayerId: id,
			Transforms: transformList,
		}

		log.Println(serverUpdate)

		serializedUpdate, err := json.Marshal(serverUpdate)
		if err != nil {
			log.Println("Failed to serialize", serializedUpdate)
			return
		}

		log.Println(string(serializedUpdate))

		n, err := websocket.Write(serializedUpdate)
		if err != nil {
			log.Println("Error Sending:", err)
			// TODO - we need to tag this entity with a logout message or something
			ecs.Delete(engine, id)
			return
		}
		log.Println("Sent n Bytes:", n)
	})
}
