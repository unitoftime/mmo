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

func ClientReceive(engine *ecs.Engine, conn net.Conn) {
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

		id := serverUpdate.Id
		transform := physics.Transform{}
		ok := ecs.Read(engine, id, &transform)
		if ok {
			// We were able to find this person
			// Then snap to their correct position
		} else {
			body := Body{}
			ecs.Write(engine, id, body)
		}

		ecs.Write(engine, id, serverUpdate.Transform)
	}
}

type ServerUpdate struct {
	Id ecs.Id
	Transform physics.Transform
}

func ServerSendUpdate(engine *ecs.Engine) {
	ecs.Each(engine, Websocket{}, func(id ecs.Id, a interface{}) {
		websocket := a.(Websocket)


		transform := physics.Transform{}
		ok := ecs.Read(engine, id, &transform)
		if !ok { return }

		serverUpdate := ServerUpdate{
			Id: id,
			Transform: transform,
		}

		// log.Println(serverUpdate)

		serializedUpdate, err := json.Marshal(serverUpdate)
		if err != nil {
			log.Println("Failed to serialize", serializedUpdate)
			return
		}

		n, err := websocket.Write(serializedUpdate)
		if err != nil {
			log.Println("Error Sending:", err)
			ecs.Delete(engine, id)
			return
		}
		log.Println("Sent n Bytes:", n)
	})
}
