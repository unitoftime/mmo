package mmo

import (
	"fmt"
	"log"
	"net"
	"bytes"
	"encoding/json"
	"encoding/binary"

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
			transform, err := serverUpdate.Transforms[i].GetTransform()
			if err != nil {
				log.Println(err)
				continue
			}

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

		transformUpdate, err := NewTransformUpdate(id, transform)
		if err != nil {
			log.Println(err)
			return
		}
		
		transformList = append(transformList, transformUpdate)
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

		serializedUpdate, err := json.Marshal(serverUpdate)
		if err != nil {
			log.Println("Failed to serialize", serializedUpdate)
			return
		}

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
	// Transform physics.Transform
	TransformBytes []byte
}

func NewTransformUpdate(id ecs.Id, transform physics.Transform) (TransformUpdate, error) {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, binary.LittleEndian, transform)
	if err != nil {
		return TransformUpdate{}, fmt.Errorf("Failed to serialize transform: %s", err)
	}

	update := TransformUpdate{
		Id: id,
		TransformBytes: buf.Bytes(),
	}
	return update, nil
}

func (t *TransformUpdate) GetTransform() (physics.Transform, error) {
	buf := bytes.NewReader(t.TransformBytes)
	transform := physics.Transform{}
	err := binary.Read(buf, binary.LittleEndian, &transform)
	if err != nil {
		return physics.Transform{}, fmt.Errorf("Failed to deserialize transform: %s", err)
	}
	return transform, nil
}
