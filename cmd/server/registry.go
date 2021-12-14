package main

import (
	"fmt"
	"github.com/unitoftime/ecs"
	"github.com/unitoftime/mmo"
	// "github.com/unitoftime/mmo/engine/render"
	"github.com/unitoftime/flow/physics"
	// "github.com/unitoftime/mmo/engine/tilemap"
)

func init() {
	ecs.SetRegistry(&ComponentRegistry{})
}

type ComponentRegistry struct {}
func (r *ComponentRegistry) GetArchStorageType(component interface{}) ecs.ArchComponent {
	switch component.(type) {
	case physics.Transform:
		return &physics.TransformList{}
	case *physics.Transform:
		return &physics.TransformList{}
	case physics.Input:
		return &physics.InputList{}
	case *physics.Input:
		return &physics.InputList{}
	case mmo.Body:
		return &mmo.BodyList{}
	case *mmo.Body:
		return &mmo.BodyList{}
	case mmo.ClientOwned:
		return &mmo.ClientOwnedList{}
	case *mmo.ClientOwned:
		return &mmo.ClientOwnedList{}
	case mmo.User:
		return &mmo.UserList{}
	case *mmo.User:
		return &mmo.UserList{}
	default:
		panic(fmt.Sprintf("Unknown component type: %T", component))
	}
}
func (r *ComponentRegistry) GetComponentMask(component interface{}) ecs.ArchMask {
	switch component.(type) {
	case physics.Transform:
		return ecs.ArchMask(1 << 0)
	case physics.Input:
		return ecs.ArchMask(1 << 1)
	case mmo.Body:
		return ecs.ArchMask(1 << 2)
	case mmo.ClientOwned:
		return ecs.ArchMask(1 << 3)
	case mmo.User:
		return ecs.ArchMask(1 << 4)
	default:
		panic(fmt.Sprintf("Unknown component type: %T", component))
	}
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// type TypeList []Type
// func (t *TypeList) ComponentSet(val interface{}) { *t = *val.(*TypeList) }
// func (t *TypeList) InternalRead(index int, val interface{}) { *val.(*Type) = (*t)[index] }
// func (t *TypeList) InternalWrite(index int, val interface{}) { (*t)[index] = *val.(*Type) }
// func (t *TypeList) InternalAppend(val interface{}) { (*t) = append((*t), val.(Type)) }
// func (t *TypeList) InternalPointer(index int) interface{} { return &(*t)[index] }
// func (t *TypeList) InternalReadVal(index int) interface{} { return (*t)[index] }
// func (t *TypeList) Delete(index int) {
// 	lastVal := (*t)[len(*t)-1]
// 	(*t)[index] = lastVal
// 	(*t) = (*t)[:len(*t)-1]
// }
// func (t *TypeList) Len() int { return len(*t) }


