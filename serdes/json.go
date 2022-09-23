package serdes

import (
	"fmt"
	"encoding/json"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/mmo/game"
)

type JsonWorldUpdate struct {
	UserId uint64
	WorldData map[ecs.Id][]JsonComponent
	Delete []ecs.Id
}

type JsonMessage struct {
	Type MessageType
	Data json.RawMessage
}

func (w *WorldUpdate) ToJson() (JsonWorldUpdate, error) {
	wu := JsonWorldUpdate{
		UserId: w.UserId,
		WorldData: make(map[ecs.Id][]JsonComponent),
		Delete: w.Delete,
	}
	for id := range w.WorldData {
		cSlice := make([]JsonComponent, 0)
		for _, c := range w.WorldData[id] {
			jComponent, err := ToJsonComponent(c)
			if err != nil { return JsonWorldUpdate{}, err }
			cSlice = append(cSlice, jComponent)
		}
		wu.WorldData[id] = cSlice
	}
	return wu, nil
}
func (w *JsonWorldUpdate) ToNormal() (WorldUpdate, error) {
	u := WorldUpdate{
		UserId: w.UserId,
		WorldData: make(map[ecs.Id][]ecs.Component),
		Delete: w.Delete,
	}

	for id := range w.WorldData {
		cSlice := make([]ecs.Component, 0)

		for _, c := range w.WorldData[id] {
			// log.Println(c.Type)
			switch c.Type {
			case "Transform":
				v := physics.Transform{}
				err := json.Unmarshal(c.Data, &v)
				if err != nil { return u, err }
				cSlice = append(cSlice, ecs.C(v))
			case "Input":
				v := physics.Input{}
				err := json.Unmarshal(c.Data, &v)
				if err != nil { return u, err }
				cSlice = append(cSlice, ecs.C(v))
			case "Body":
				v := game.Body{}
				err := json.Unmarshal(c.Data, &v)
				if err != nil { return u, err }
				cSlice = append(cSlice, ecs.C(v))
			default:
				return u, fmt.Errorf("Unknown Component %v", c)
			}
		}
		u.WorldData[id] = cSlice
	}
	return u, nil
}

type JsonComponent struct {
	Type string
	Data json.RawMessage
}
func ToJsonComponent(v any) (JsonComponent, error) {
	var name string
	var cData []byte
	var err error
	switch c := v.(type) {
	case ecs.CompBox[physics.Transform]:
		name = c.Name()
		cData, err = json.Marshal(c.Get())
		if err != nil { return JsonComponent{}, err }
	case ecs.CompBox[physics.Input]:
		name = c.Name()
		cData, err = json.Marshal(c.Get())
		if err != nil { return JsonComponent{}, err }
	case ecs.CompBox[game.Body]:
		name = c.Name()
		cData, err = json.Marshal(c.Get())
		if err != nil { return JsonComponent{}, err }
	default:
		return JsonComponent{}, fmt.Errorf("Unknown component %t", c)
	}

	comp := JsonComponent{
		Type: name,
		Data: cData,
	}
	return comp, err
}

func (s *Serdes) JsonMarshal(v any) ([]byte, error) {
	switch t := v.(type) {
	case WorldUpdate:
		wu, err := t.ToJson()
		if err != nil { return nil, err }
		dat, err := json.Marshal(wu)
		if err != nil { return nil, err }
		return json.Marshal(JsonMessage{WorldUpdateType, dat})
	case ClientLogin:
		dat, err := json.Marshal(t)
		if err != nil { return nil, err }
		return json.Marshal(JsonMessage{ClientLoginType, dat})
	case ClientLoginResp:
		dat, err := json.Marshal(t)
		if err != nil { return nil, err }
		return json.Marshal(JsonMessage{ClientLoginRespType, dat})
	case ClientLogout:
		dat, err := json.Marshal(t)
		if err != nil { return nil, err }
		return json.Marshal(JsonMessage{ClientLogoutType, dat})
	case ClientLogoutResp:
		dat, err := json.Marshal(t)
		if err != nil { return nil, err }
		return json.Marshal(JsonMessage{ClientLogoutRespType, dat})
	}

	return nil, fmt.Errorf("Unknown Data Type %T", v) }
}

func (s *Serdes) JsonUnmarshal(dat []byte) (any, error) {
	// log.Println("Unmarshal")
	// log.Println(string(dat))

	msg := JsonMessage{}
	err := json.Unmarshal(dat, &msg)
	if err != nil { return nil, fmt.Errorf("Failed to unmarshal JsonMessage: %w", err) }

	switch msg.Type {
	case WorldUpdateType:
		dat := JsonWorldUpdate{}
		err := json.Unmarshal(msg.Data, &dat)
		if err != nil { return nil, err }
		return dat.ToNormal()
	case ClientLoginType:
		dat := ClientLogin{}
		err := json.Unmarshal(msg.Data, &dat)
		return dat, err
	case ClientLoginRespType:
		dat := ClientLoginResp{}
		err := json.Unmarshal(msg.Data, &dat)
		return dat, err
	case ClientLogoutType:
		dat := ClientLogout{}
		err := json.Unmarshal(msg.Data, &dat)
		return dat, err
	case ClientLogoutRespType:
		dat := ClientLogoutResp{}
		err := json.Unmarshal(msg.Data, &dat)
		return dat, err
	}

	return nil, fmt.Errorf("Unknown Message type %d", msg.Type) }
}
