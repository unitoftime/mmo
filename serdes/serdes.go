package serdes

import (
	"fmt"

	"bytes"
	"encoding/gob"
	"sync"

	"github.com/unitoftime/ecs"
)

// Quick test
// binary: 187 Kb/s
// Gob:    283 Kb/s
// Flatbu: 304 Kb/s
// Json:   411 Kb/s

type MessageType uint8
const (
	WorldUpdateType MessageType = iota
	ClientLoginType
	ClientLoginRespType
	ClientLogoutType
	ClientLogoutRespType
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

type Serdes struct {
	encMut, decMut *sync.Mutex

	GobEncoder *gob.Encoder
	EncBuf *bytes.Buffer
	GobDecoder *gob.Decoder
	DecBuf *bytes.Buffer
	Method string // Flatbuffers, gob, custom binary, etc
}

func New() *Serdes {
	var encBuf, decBuf bytes.Buffer
	return &Serdes{
		Method: "gob",

		GobEncoder: gob.NewEncoder(&encBuf),
		EncBuf: &encBuf,
		GobDecoder: gob.NewDecoder(&decBuf),
		DecBuf: &decBuf,
		encMut: &sync.Mutex{},
		decMut: &sync.Mutex{},
	}
}

func (s *Serdes) Marshal(v any) ([]byte, error) {
	switch s.Method {
	case "fb":
		return s.FlatbufferMarshal(v)
	case "json":
		return s.JsonMarshal(v)
	case "gob":
		return s.GobMarshal(v)
	case "binary":
		return MarshalBinary(v)
	}
	panic(fmt.Sprintf("Unknown method type: %s", s.Method))
}

func (s *Serdes) Unmarshal(dat []byte) (any, error) {
	switch s.Method {
	case "fb":
		return s.FlatbufferUnmarshal(dat)
	case "json":
		return s.JsonUnmarshal(dat)
	case "gob":
		return s.GobUnmarshal(dat)
	case "binary":
		return UnmarshalBinary(dat)
	}
	panic(fmt.Sprintf("Unknown method type: %s", s.Method))
}
