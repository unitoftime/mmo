package serdes

import (
	"fmt"
	"reflect"

	"github.com/unitoftime/binary"

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

	return nil, fmt.Errorf("Unknown message header %v", msg)
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

type ComponentType uint8
const (
	PhysicsTransform ComponentType = iota
	PhysicsInput
	GameBody
	GameSpeech
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
	case ecs.CompBox[game.Speech]:
		header = GameSpeech
		cData, err = binary.Marshal(c.Get())
		if err != nil { return BinaryComponent{}, err }
	default:
		return BinaryComponent{}, fmt.Errorf("Unknown component %T", c)
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
	case GameSpeech:
		v := game.Speech{}
		err := binary.Unmarshal(b.Payload, &v)
		if err != nil { return nil, err }
		return ecs.C(v), nil
	}

	return nil, fmt.Errorf("Unknown Message header %v", b)
}


type BinWorldUpdate struct {
	Tick uint16
	PlayerTick uint16
	UserId uint64
	WorldData map[uint32][]BinaryComponent
	Delete []ecs.Id
	// Messages []ChatMessage
}

func (w WorldUpdate) MarshalBinary() ([]byte, error) {
	wu := BinWorldUpdate{
		Tick: w.Tick,
		PlayerTick: w.PlayerTick,
		UserId: w.UserId,
		// WorldData: make(map[ecs.Id][]BinaryComponent), // TODO the binary serdes package I'm using doesn't support ecs.Id as a key panic: reflect.Value.SetMapIndex: value of type uint32 is not assignable to type ecs.Id [recovered] panic: reflect.Value.SetMapIndex: value of type uint32 is not assignable to type ecs.Id
		WorldData: make(map[uint32][]BinaryComponent),
		Delete: w.Delete,
		// Messages: w.Messages,
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

	w.Tick = wu.Tick
	w.PlayerTick = wu.PlayerTick
	w.UserId = wu.UserId
	w.Delete = wu.Delete
	// w.Messages = wu.Messages
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

type Union struct {
	Type uint8
	Payload []byte
}

// func NewUnionBuilder(structs ...any) Union {
// 	for i := range structs {
// 	}
// }

// TODO - long term do something like this
// type UnionDefinition struct {
// 	WorldUpdateMsg *WorldUpdate `type:1`
// 	ClientLoginMsg *ClientLogin `type:0`
// 	ClientLoginResp *ClientLoginResp `type:2`
// 	ClientLogoutMsg *ClientLogout `type:3`
// 	ClientLogoutResp *ClientLogoutResp `type:4`
// }

// TODO - handle pointer stucts here
// func typeString(val any) string {
// 	ty := reflect.TypeOf(val)
// 	return ty.PkgPath() + "." + ty.Name()
// }

func NewUnion(structs ...any) *UnionBuilder {
	if len(structs) > 256 {
		panic("TOO MANY STRUCTS")
	}

	types := make(map[reflect.Type]uint8)
	for i := range structs {
		// typeStr := typeString(structs[i])
		// fmt.Println(typeStr, uint8(i))
		typeStr := reflect.TypeOf(structs[i])
		// fmt.Println(typeStr, uint8(i))
		types[typeStr] = uint8(i)
	}
	// fmt.Println(types)
	return &UnionBuilder {
		types: types,
		impl: structs,
	}
}

type UnionBuilder struct {
	// types map[string]uint8
	types map[reflect.Type]uint8
	// def UnionDefinition
	impl []any
}

// Converts the underlying value inside the to a pointer and returns an interface for that
func valToPtr(val any) any {
	v := reflect.ValueOf(val)
	rVal := reflect.New(v.Type())
	rVal.Elem().Set(v)
	ptrVal := rVal.Interface()
	return ptrVal
}
// Converts the underlying interface with pointer to just the value
func ptrToVal(valPtr any) any {
	return reflect.Indirect(reflect.ValueOf(valPtr)).Interface()
}


func (u *UnionBuilder) Serialize(val any) ([]byte, error) {
	// ptrVal := valToPtr(val)
	// fmt.Printf("HERE: %T %T, %v, %v\n", val, ptrVal, val, ptrVal)

	typeStr := reflect.TypeOf(val)
	typeId, ok := u.types[typeStr]
	if !ok {
		return nil, fmt.Errorf("Unknown Type: %T %T", val, val)
	}

	// TODO - can optimize the double serialize
	serializedVal, err := binary.Marshal(val)
	if err != nil {
		return nil, err
	}
	union := Union{
		Type: typeId,
		Payload: serializedVal,
	}

	serializedUnion, err := binary.Marshal(union)
	return serializedUnion, err
}

func (u *UnionBuilder) Deserialize(dat []byte) (any, error) {
	union := Union{}
	err := binary.Unmarshal(dat, &union)
	if err != nil { return nil, err }
	// fmt.Printf("Deserialize: %v\n", union)

	val := u.impl[int(union.Type)]
	valPtr := valToPtr(val)
	// fmt.Printf("valtoptr: %T, %T\n", val, valPtr)

	err = binary.Unmarshal(union.Payload, valPtr)
	// fmt.Println(union, val, union.Type, union.Payload, len(union.Payload))

	return ptrToVal(valPtr), err
}

// ---

// type Union struct {
// 	Type uint8
// 	Data []byte
// }


// type Union struct {
// 	worldUpdate *WorldUpdate
// 	clientLogin *ClientLogin
// 	clientLogout *ClientLogout
// 	clientLoginResp *ClientLoginResp
// 	clientLogoutResp *ClientLogoutResp
// }

// func (u *Union) Set(val any) {
// 	// Check to make sure all are nil

// 	// If all are nil, then set the appropriate one
// 	switch t := val.(type) {
// 	WorldUpdate:
		
// 	}
// }

// func (u *Union) Get(val any) {
// 	// checks to find the non-nil value

// 	// pulls the value out and set val to it
// }
