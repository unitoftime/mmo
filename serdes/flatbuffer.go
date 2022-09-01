package serdes

import (
	"fmt"

	flatbuffers "github.com/google/flatbuffers/go"
	flatmsg "github.com/unitoftime/mmo/flat/msg"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/mmo/game"
)

type WorldUpdate struct {
	UserId uint64
	WorldData map[ecs.Id][]ecs.Component
	Delete []ecs.Id
}

type ClientLogin struct {
	UserId uint64
}

type ClientLoginResp struct {
	UserId uint64
	Id ecs.Id
}

type ClientLogout struct {
	UserId uint64
}
type ClientLogoutResp struct {
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

func MarshalClientLogoutMessage(userId uint64) []byte {
	builder := flatbuffers.NewBuilder(1024)
	flatmsg.ClientLogoutStart(builder)
	flatmsg.ClientLogoutAddUserId(builder, userId)
	clientLogout := flatmsg.ClientLogoutEnd(builder)

	flatmsg.MessageStart(builder)
	flatmsg.MessageAddPayloadType(builder, flatmsg.PayloadClientLogout)
	flatmsg.MessageAddPayload(builder, clientLogout)
	msg := flatmsg.MessageEnd(builder)

	builder.Finish(msg)

	buf := builder.FinishedBytes()
	return buf
}

func MarshalClientLogoutRespMessage(userId uint64, id ecs.Id) []byte {
	builder := flatbuffers.NewBuilder(1024)
	flatmsg.ClientLogoutRespStart(builder)
	flatmsg.ClientLogoutRespAddUserId(builder, userId)
	flatmsg.ClientLogoutRespAddId(builder, uint32(id))
	clientLogoutResp := flatmsg.ClientLogoutRespEnd(builder)

	flatmsg.MessageStart(builder)
	flatmsg.MessageAddPayloadType(builder, flatmsg.PayloadClientLogoutResp)
	flatmsg.MessageAddPayload(builder, clientLogoutResp)
	msg := flatmsg.MessageEnd(builder)

	builder.Finish(msg)

	buf := builder.FinishedBytes()
	return buf
}

func MarshalWorldUpdateMessage(update WorldUpdate) ([]byte, error) {
	builder := flatbuffers.NewBuilder(1024)

	worldData := update.WorldData
	userId := update.UserId
	toDelete := update.Delete

	entList := make([]flatbuffers.UOffsetT, 0)
	for id, compList := range worldData {
		flatmsg.EntityStart(builder)
		flatmsg.EntityAddId(builder, uint32(id))

		for _,comp := range compList {
			switch t := comp.(type) {
			case ecs.CompBox[physics.Transform]:
				transform := t.Get()
				transformMsg := flatmsg.CreateTransform(builder, transform.X, transform.Y)
				flatmsg.EntityAddTransform(builder, transformMsg)
			case ecs.CompBox[physics.Input]:
				input := t.Get()
				inputMsg := flatmsg.CreateInput(builder,
					input.Up, input.Down, input.Left, input.Right)
				flatmsg.EntityAddInput(builder, inputMsg)
			case ecs.CompBox[game.Body]:
				body := t.Get()
				bodyMsg := flatmsg.CreateBody(builder,
					body.Type)
				flatmsg.EntityAddBody(builder, bodyMsg)
			default:
				return nil, fmt.Errorf("Unknown component %t", comp)
			}
		}

		entList = append(entList, flatmsg.EntityEnd(builder))
	}

	flatmsg.WorldUpdateStartEntitiesVector(builder, len(entList))
	for i := range entList {
		builder.PrependUOffsetT(entList[i])
	}
	entities := builder.EndVector(len(entList))

	flatmsg.WorldUpdateStartDeleteVector(builder, len(toDelete))
	for i := range toDelete {
		builder.PrependUint32(uint32(toDelete[i]))
	}
	delete := builder.EndVector(len(toDelete))

	flatmsg.WorldUpdateStart(builder)
	flatmsg.WorldUpdateAddUserId(builder, userId)
	flatmsg.WorldUpdateAddEntities(builder, entities)
	flatmsg.WorldUpdateAddDelete(builder, delete)
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

		worldData := make(map[ecs.Id][]ecs.Component)

		length := worldUpdate.EntitiesLength()
		for i := 0; i < length; i++ {
			entity := &flatmsg.Entity{}
			ok := worldUpdate.Entities(entity, i)
			if !ok {
				return nil, fmt.Errorf("Unable to access entity at index %d", i)
			}

			compList := make([]ecs.Component, 0)

			transform := entity.Transform(nil)
			if transform != nil {
				// log.Println(entity.Transform(nil).X())
				// log.Println(entity.Transform(nil).Y())
				compList = append(compList, ecs.C(physics.Transform{
					X: transform.X(),
					Y: transform.Y(),
				}))
			}

			input := entity.Input(nil)
			if input != nil {
				// log.Println("Input", input.Up(), input.Down(), input.Left(), input.Right())
				compList = append(compList, ecs.C(physics.Input{
					Up: input.Up(),
					Down: input.Down(),
					Left: input.Left(),
					Right: input.Right(),
				}))
			}

			body := entity.Body(nil)
			if body != nil {
				compList = append(compList, ecs.C(game.Body{
					Type: body.Val(),
				}))
			}

			// log.Println(entity.Id())
			worldData[ecs.Id(entity.Id())] = compList
		}

		delete := make([]ecs.Id, 0)
		deleteLength := worldUpdate.DeleteLength()
		for i := 0; i < deleteLength; i++ {
			id := worldUpdate.Delete(i)
			// TODO - there doesn't appear to be any checks to show that it succeeded. Idk what happens if we go out of bounds, so I'll just check if it's zeroed here
			if ecs.Id(id) == ecs.InvalidEntity {
				return nil, fmt.Errorf("Unable to access deleteList at index %d", i)
			}

			delete = append(delete, ecs.Id(id))
		}

		msg := WorldUpdate{
			UserId: worldUpdate.UserId(),
			WorldData: worldData,
			Delete: delete,
		}
		// fmt.Println("Unmarshal", msg)
		return msg, nil

	// --- Login ---
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

	// --- Logout ---
	case flatmsg.PayloadClientLogout:
		clientLogout := new(flatmsg.ClientLogout)
		clientLogout.Init(payloadUnion.Bytes, payloadUnion.Pos)
		return ClientLogout{
			UserId: clientLogout.UserId(), // TODO - can this be null?
		}, nil

	case flatmsg.PayloadClientLogoutResp:
		clientLogoutResp := new(flatmsg.ClientLogoutResp)
		clientLogoutResp.Init(payloadUnion.Bytes, payloadUnion.Pos)
		return ClientLogoutResp{
			UserId: clientLogoutResp.UserId(), // TODO - can this be null?
			Id: ecs.Id(clientLogoutResp.Id()), // TODO - can this be null?
		}, nil
	default:
		return nil, fmt.Errorf("Unknown Flatbuffer message payload type %s", payloadType)
	}

	return nil, nil
}

