package serdes

import (
	"fmt"
	"encoding/gob"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/flow/physics"
	"github.com/unitoftime/mmo/game"
)

func init() {
	gob.Register(WorldUpdate{})
	gob.Register(ClientLogin{})
	gob.Register(ClientLoginResp{})
	gob.Register(ClientLogout{})
	gob.Register(ClientLogoutResp{})

	gob.Register(ecs.C(physics.Transform{}))
	gob.Register(ecs.C(physics.Input{}))
	gob.Register(ecs.C(game.Body{}))

	gob.Register(physics.Transform{})
	gob.Register(physics.Input{})
	gob.Register(game.Body{})
}

type GobMessage struct {
	Data any
}

func (s *Serdes) GobMarshal(v any) ([]byte, error) {
	s.encMut.Lock()
	defer func() {
		s.encMut.Unlock()
	}()

	msg := GobMessage{v}

	s.EncBuf.Reset() // Clear the buffer before we encode
	err := s.GobEncoder.Encode(msg)
	if err != nil {
		fmt.Println("GobError Marshal", err)
	}

	// Note: The returned byte buffer is only valid until the next marshal call
	return s.EncBuf.Bytes(), err
}

func (s *Serdes) GobUnmarshal(dat []byte) (any, error) {
	s.decMut.Lock()
	defer func() {
		s.decMut.Unlock()
	}()
	// fmt.Println("Dat:", string(dat))

	s.DecBuf.Write(dat) // Write our data to the buffer
	msg := GobMessage{}
	err := s.GobDecoder.Decode(&msg)
	s.DecBuf.Reset() // Clear the buffer after we've decoded the data
	if err != nil {
		fmt.Println("GobError Unmarshal", err)
	}

	// Note: The returned byte buffer is only valid until the next marshal call
	return msg.Data, err
}
