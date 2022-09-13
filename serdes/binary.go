package serdes

import (
	"fmt"

	"github.com/kelindar/binary"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/mmo/game"
)

// TODO - for delta encoding of things that have to be different like ecs.Ids, if you encode the number as 0 then that could indicate that "we needed more bytes to encode the delta"

// TODO - rename "BinaryMessage"? or binary union and combine with the unioning of components
type PackedMessage struct {
	Header MessageType
	Payload []byte // TODO - this is inefficient because its a slice, not an array. I technically don't need variable length here because the length is defined by the header
}

func MarshalBinary(v any) ([]byte, error) {
	var msgType MessageType

	switch v.(type) {
	case WorldUpdate:
		msgType = WorldUpdateType
	case ClientLogin:
		msgType = ClientLoginType
	case ClientLoginResp:
		msgType = ClientLoginRespType
	case ClientLogout:
		msgType = ClientLogoutType
	case ClientLogoutResp:
		msgType = ClientLogoutRespType
	}

	payload, err := binary.Marshal(v)
	if err != nil { return nil, err }

	return binary.Marshal(PackedMessage{
		Header: msgType,
		Payload: payload,
	})
}

func UnmarshalBinary(dat []byte) (any, error) {
	msg := PackedMessage{}
	err := binary.Unmarshal(dat, &msg)
	if err != nil { return nil, err }

	switch msg.Header {
	case WorldUpdateType:
		ret := WorldUpdate{}
		err := binary.Unmarshal(msg.Payload, &ret)
		return ret, err
	case ClientLoginType:
		ret := ClientLogin{}
		err := binary.Unmarshal(msg.Payload, &ret)
		return ret, err
	case ClientLoginRespType:
		ret := ClientLoginResp{}
		err := binary.Unmarshal(msg.Payload, &ret)
		return ret, err
	case ClientLogoutType:
		ret := ClientLogout{}
		err := binary.Unmarshal(msg.Payload, &ret)
		return ret, err
	case ClientLogoutRespType:
		ret := ClientLogoutResp{}
		err := binary.Unmarshal(msg.Payload, &ret)
		return ret, err
	}

	panic("UNKNOWN")
}

// type Message struct {
// 	Type MessageType
// 	Data any
// }

// func (m Message) MarshalBinary() ([]byte, error) {
// 	switch m.Type {
// 	case WorldUpdateType:
// 	case ClientLoginType:
// 		msg := TaggedUnion[MessageType, ClientLogin]{
// 			Type: m.Type,
// 			Data: m.Data.(ClientLogin),
// 		}
// 		return binary.Marshal(msg)
// 	case ClientLoginRespType:
// 	case ClientLogoutType:
// 	case ClientLogoutRespType:

// 	}
// 	panic("Unknown type")
// }

// func (m Message) UnmarshalBinary() ([]byte, error) {
// 	switch m.Type {
// 	case WorldUpdateType:
// 	case ClientLoginType:
// 		msg := TaggedUnion[MessageType, ClientLogin]{
// 			Type: m.Type,
// 			Data: m.Data.(ClientLogin),
// 		}
// 		return binary.Marshal(msg)
// 	case ClientLoginRespType:
// 	case ClientLogoutType:
// 	case ClientLogoutRespType:

// 	}
// 	panic("Unknown type")
// }

type TaggedUnion[T comparable] struct {
	Header T
	Payload []byte
}

type ComponentType uint8
const (
	PhysicsTransform ComponentType = iota
	PhysicsInput
	GameBody
	// TODO - etc...
)

type BinaryComponent struct {
	Header ComponentType
	Payload []byte
}
func NewBinaryComponent(v any) (BinaryComponent, error) {
	var header ComponentType
	var cData []byte
	var err error
	switch c := v.(type) {
	case ecs.CompBox[physics.Transform]:
		header = PhysicsTransform
		cData, err = binary.Marshal(c.Get())
		if err != nil { return BinaryComponent{}, err }
	case ecs.CompBox[physics.Input]:
		header = PhysicsInput

		cData, err = binary.Marshal(c.Get())
		if err != nil { return BinaryComponent{}, err }
	case ecs.CompBox[game.Body]:
		header = GameBody
		cData, err = binary.Marshal(c.Get())
		if err != nil { return BinaryComponent{}, err }
	default:
		return BinaryComponent{}, fmt.Errorf("Unknown component %t", c)
	}

	comp := BinaryComponent{
		Header: header,
		Payload: cData,
	}
	return comp, err
}
func (b *BinaryComponent) ToNormal() (ecs.Component, error) {
	switch b.Header {
	case PhysicsTransform:
		v := physics.Transform{}
		err := binary.Unmarshal(b.Payload, &v)
		if err != nil { return nil, err }
		return ecs.C(v), nil
	case PhysicsInput:
		v := physics.Input{}
		err := binary.Unmarshal(b.Payload, &v)
		if err != nil { return nil, err }
		return ecs.C(v), nil
	case GameBody:
		v := game.Body{}
		err := binary.Unmarshal(b.Payload, &v)
		if err != nil { return nil, err }
		return ecs.C(v), nil
	}
	panic("UNKNOWN")
}


type BinWorldUpdate struct {
	UserId uint64
	WorldData map[uint32][]BinaryComponent
	Delete []ecs.Id
}

func (w WorldUpdate) MarshalBinary() ([]byte, error) {
	wu := BinWorldUpdate{
		UserId: w.UserId,
		// WorldData: make(map[ecs.Id][]BinaryComponent), // TODO the binary serdes package I'm using doesn't support ecs.Id as a key panic: reflect.Value.SetMapIndex: value of type uint32 is not assignable to type ecs.Id [recovered] panic: reflect.Value.SetMapIndex: value of type uint32 is not assignable to type ecs.Id
		WorldData: make(map[uint32][]BinaryComponent),
		Delete: w.Delete,
	}
	for id := range w.WorldData {
		cSlice := make([]BinaryComponent, 0)
		for _, c := range w.WorldData[id] {
			binComponent, err := NewBinaryComponent(c)
			if err != nil { return nil, err }
			cSlice = append(cSlice, binComponent)
		}
		wu.WorldData[uint32(id)] = cSlice
	}

	return binary.Marshal(wu)
}

func (w *WorldUpdate) UnmarshalBinary(data []byte) error {
	wu := BinWorldUpdate{}
	err := binary.Unmarshal(data, &wu)
	if err != nil { return nil }

	w.UserId = wu.UserId
	w.Delete = wu.Delete
	if w.WorldData == nil {
		w.WorldData = make(map[ecs.Id][]ecs.Component)
	}

	for id := range wu.WorldData {
		cSlice := make([]ecs.Component, 0)
		for _, c := range wu.WorldData[id] {
			comp, err := c.ToNormal()
			if err != nil { return err }
			cSlice = append(cSlice, comp)
		}
		w.WorldData[ecs.Id(id)] = cSlice
	}

	return nil
}
