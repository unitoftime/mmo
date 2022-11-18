package serdes

import (
	"fmt"
	"reflect"

	"github.com/unitoftime/binary"
)

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

func (u *UnionBuilder) Make(val any) (Union, error) {
	typeStr := reflect.TypeOf(val)
	typeId, ok := u.types[typeStr]
	if !ok {
		return Union{}, fmt.Errorf("Unknown Type: %T %T", val, val)
	}

	// TODO - can optimize the double serialize
	serializedVal, err := binary.Marshal(val)
	if err != nil {
		return Union{}, err
	}
	union := Union{
		Type: typeId,
		Payload: serializedVal,
	}
	return union, nil
}

func (u *UnionBuilder) Unmake(union Union) (any, error) {
	val := u.impl[int(union.Type)]
	valPtr := valToPtr(val)
	// fmt.Printf("valtoptr: %T, %T\n", val, valPtr)

	err := binary.Unmarshal(union.Payload, valPtr)
	// fmt.Println(union, val, union.Type, union.Payload, len(union.Payload))

	return ptrToVal(valPtr), err
}

func (u *UnionBuilder) Serialize(val any) ([]byte, error) {
	// ptrVal := valToPtr(val)
	// fmt.Printf("HERE: %T %T, %v, %v\n", val, ptrVal, val, ptrVal)

	union, err := u.Make(val)
	if err != nil {
		return nil, err
	}

	serializedUnion, err := binary.Marshal(union)
	return serializedUnion, err
}

func (u *UnionBuilder) Deserialize(dat []byte) (any, error) {
	union := Union{}
	err := binary.Unmarshal(dat, &union)
	if err != nil { return nil, err }
	// fmt.Printf("Deserialize: %v\n", union)

	return u.Unmake(union)
}

