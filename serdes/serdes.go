package serdes

import (
	"github.com/unitoftime/binary"
	"github.com/unitoftime/ecs"

	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/mmo/game"
)

// type MessageRouter struct {
// 	handlers map[reflect.Type]func(any) (error)
// }

// type ComponentRouter struct {
// 	handlers map[reflect.Type]func(any) (ecs.Component, error)
// }

// Quick test
// binary: 187 Kb/s
// Gob:    283 Kb/s
// Flatbu: 304 Kb/s
// Json:   411 Kb/s

// TODO! - should I just have one big union object that everything is in? That'll greatly simplify a recursive serializer. Kindoflike gob where if you hit an interface you just try to unionize it. Then when you pull it out you do the opposite...
var componentUnion *UnionBuilder
func init() {
	// componentUnion = NewUnion(physics.Transform{}, physics.Input{}, game.Body{}, game.Speech{})
	componentUnion = NewUnion(ecs.C(physics.Transform{}), ecs.C(physics.Input{}), ecs.C(game.Body{}), ecs.C(game.Speech{}))
}

// TODO - for delta encoding of things that have to be different like ecs.Ids, if you encode the number as 0 then that could indicate that "we needed more bytes to encode the delta"
type WorldUpdate struct {
	Tick uint16
	PlayerTick uint16
	UserId uint64
	WorldData map[ecs.Id][]ecs.Component
	// WorldData EntityMap // TODO - might be nice to reduce the BinWorldUpdate to just the entity map
	Delete []ecs.Id
}
type BinWorldUpdate struct {
	Tick uint16
	PlayerTick uint16
	UserId uint64
	WorldData map[uint32][]Union
	Delete []ecs.Id
}

func (w WorldUpdate) MarshalBinary() ([]byte, error) {
	wu := BinWorldUpdate{
		Tick: w.Tick,
		PlayerTick: w.PlayerTick,
		UserId: w.UserId,
		// WorldData: make(map[ecs.Id][]BinaryComponent), // TODO the binary serdes package I'm using doesn't support ecs.Id as a key panic: reflect.Value.SetMapIndex: value of type uint32 is not assignable to type ecs.Id [recovered] panic: reflect.Value.SetMapIndex: value of type uint32 is not assignable to type ecs.Id
		WorldData: make(map[uint32][]Union),
		Delete: w.Delete,
	}
	for id := range w.WorldData {
		cSlice := make([]Union, 0)
		for _, c := range w.WorldData[id] {
			union, err := componentUnion.Make(c)
			if err != nil { return nil, err }
			cSlice = append(cSlice, union)
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
		for _, union := range wu.WorldData[id] {
			anyComp, err := componentUnion.Unmake(union)
			if err != nil { return err }
			// comp := toComponent(anyComp)
			comp, ok := anyComp.(ecs.Component)
			if ok {
				cSlice = append(cSlice, comp)
			}
		}
		w.WorldData[ecs.Id(id)] = cSlice
	}

	return nil
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

type Serdes struct {
	union *UnionBuilder
}

func New() *Serdes {
	return &Serdes{
		union: NewUnion(WorldUpdate{}, ClientLogin{}, ClientLoginResp{}, ClientLogout{}, ClientLogoutResp{}),
	}
}

func (s *Serdes) Marshal(v any) ([]byte, error) {
	return s.union.Serialize(v)
}

func (s *Serdes) Unmarshal(dat []byte) (any, error) {
	return s.union.Deserialize(dat)
}
