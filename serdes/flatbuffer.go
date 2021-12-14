package serdes

import (
	"fmt"

	flatbuffers "github.com/google/flatbuffers/go"
	flatmsg "github.com/unitoftime/mmo/flat/msg"

	"github.com/unitoftime/ecs"
	// "github.com/unitoftime/mmo"
	"github.com/unitoftime/flow/physics"
)

type WorldUpdate struct {
	UserId uint64
	WorldData map[ecs.Id][]interface{}
}

type ClientLogin struct {
	UserId uint64
}

type ClientLoginResp struct {
	UserId uint64
	Id ecs.Id
}

func MarshalClientLoginMessage(userId uint64) []byte {
	builder := flatbuffers.NewBuilder(1024)
	flatmsg.ClientLoginStart(builder)
	flatmsg.ClientLoginAddUserId(builder, userId)
	clientLogin := flatmsg.ClientLoginEnd(builder)

	flatmsg.MessageStart(builder)
	flatmsg.MessageAddPayloadType(builder, flatmsg.PayloadClientLogin)
	flatmsg.MessageAddPayload(builder, clientLogin)
	msg := flatmsg.MessageEnd(builder)

	builder.Finish(msg)

	buf := builder.FinishedBytes()
	return buf
}

func MarshalClientLoginRespMessage(userId uint64, id ecs.Id) []byte {
	builder := flatbuffers.NewBuilder(1024)
	flatmsg.ClientLoginRespStart(builder)
	flatmsg.ClientLoginRespAddUserId(builder, userId)
	flatmsg.ClientLoginRespAddId(builder, uint32(id))
	clientLoginResp := flatmsg.ClientLoginRespEnd(builder)

	flatmsg.MessageStart(builder)
	flatmsg.MessageAddPayloadType(builder, flatmsg.PayloadClientLoginResp)
	flatmsg.MessageAddPayload(builder, clientLoginResp)
	msg := flatmsg.MessageEnd(builder)

	builder.Finish(msg)

	buf := builder.FinishedBytes()
	return buf
}

func MarshalWorldUpdateMessage(update WorldUpdate) ([]byte, error) {
	builder := flatbuffers.NewBuilder(1024)

	worldData := update.WorldData
	userId := update.UserId

	entList := make([]flatbuffers.UOffsetT, 0)
	for id, compList := range worldData {
		flatmsg.EntityStart(builder)
		flatmsg.EntityAddId(builder, uint32(id))

		for _,comp := range compList {
			switch t := comp.(type) {
			case physics.Transform:
				transformMsg := flatmsg.CreateTransform(builder, t.X, t.Y)
				flatmsg.EntityAddTransform(builder, transformMsg)
			case physics.Input:
				inputMsg := flatmsg.CreateInput(builder,
					t.Up, t.Down, t.Left, t.Right)
				flatmsg.EntityAddInput(builder, inputMsg)
			// case mmo.Body:
			// 	bodyMsg := flatmsg.CreateBody(builder,
			// 		1)
			// 	flatmsg.EntityAddBody(builder, bodyMsg)
			// default:
			// 	return nil, fmt.Errorf("Unknown component %t", comp)
			}
		}

		entList = append(entList, flatmsg.EntityEnd(builder))
	}

	flatmsg.WorldUpdateStartEntitiesVector(builder, len(entList))
	for i := range entList {
		builder.PrependUOffsetT(entList[i])
	}
	entities := builder.EndVector(len(entList))

	flatmsg.WorldUpdateStart(builder)
	flatmsg.WorldUpdateAddUserId(builder, userId)
	flatmsg.WorldUpdateAddEntities(builder, entities)
	worldUpdate := flatmsg.WorldUpdateEnd(builder)

	flatmsg.MessageStart(builder)
	flatmsg.MessageAddPayloadType(builder, flatmsg.PayloadWorldUpdate)
	flatmsg.MessageAddPayload(builder, worldUpdate)
	msg := flatmsg.MessageEnd(builder)

	builder.Finish(msg)
	dat := builder.FinishedBytes()
	return dat, nil
}

///////////////////////////////////////////////////////////////////////
func UnmarshalMessage(buf []byte) (interface{}, error) {
	msg := flatmsg.GetRootAsMessage(buf, 0)
	if msg == nil {
		return nil, fmt.Errorf("Invalid message data")
	}

	payloadUnion := new(flatbuffers.Table)
	ok := msg.Payload(payloadUnion)
	if !ok {
		return nil, fmt.Errorf("Invalid payload data")
	}

	payloadType := msg.PayloadType()
	// log.Println("PayloadType:", payloadType)

	switch payloadType {
	case flatmsg.PayloadWorldUpdate:
		worldUpdate := new(flatmsg.WorldUpdate)
		worldUpdate.Init(payloadUnion.Bytes, payloadUnion.Pos)

		worldData := make(map[ecs.Id][]interface{})

		length := worldUpdate.EntitiesLength()
		for i := 0; i < length; i++ {
			entity := &flatmsg.Entity{}
			ok := worldUpdate.Entities(entity, i)
			if !ok {
				return nil, fmt.Errorf("Unable to access entity at index %d", i)
			}

			compList := make([]interface{}, 0)

			transform := entity.Transform(nil)
			if transform != nil {
				// log.Println(entity.Transform(nil).X())
				// log.Println(entity.Transform(nil).Y())
				compList = append(compList, physics.Transform{
					X: transform.X(),
					Y: transform.Y(),
				})
			}

			input := entity.Input(nil)
			if input != nil {
				// log.Println("Input", input.Up(), input.Down(), input.Left(), input.Right())
				compList = append(compList, physics.Input{
					Up: input.Up(),
					Down: input.Down(),
					Left: input.Left(),
					Right: input.Right(),
				})
			}

			// log.Println(entity.Id())
			worldData[ecs.Id(entity.Id())] = compList
		}

		msg := WorldUpdate{
			UserId: worldUpdate.UserId(),
			WorldData: worldData,
		}
		fmt.Println("Unmarshal", msg)
		return msg, nil
	case flatmsg.PayloadClientLogin:
		clientLogin := new(flatmsg.ClientLogin)
		clientLogin.Init(payloadUnion.Bytes, payloadUnion.Pos)
		return ClientLogin{
			UserId: clientLogin.UserId(), // TODO - can this be null?
		}, nil

	case flatmsg.PayloadClientLoginResp:
		clientLoginResp := new(flatmsg.ClientLoginResp)
		clientLoginResp.Init(payloadUnion.Bytes, payloadUnion.Pos)
		return ClientLoginResp{
			UserId: clientLoginResp.UserId(), // TODO - can this be null?
			Id: ecs.Id(clientLoginResp.Id()), // TODO - can this be null?
		}, nil
	default:
		return nil, fmt.Errorf("Unknown Flatbuffer message payload type %s", payloadType)
	}

	return nil, nil
}

