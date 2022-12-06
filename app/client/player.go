package client

import (
	"time"
	"math"
	"sync"

	"github.com/unitoftime/flow/ds"

	"github.com/unitoftime/ecs"
	"github.com/unitoftime/mmo"
)

type InputBufferItem struct {
	Input mmo.Input
	Time time.Time
}

// This represents global player data on the client
type PlayerData struct {
	mu sync.RWMutex
	id ecs.Id
	playerTick uint16
	serverTick uint16
	lastMessage string
	inputBuffer []InputBufferItem
	roundTripTimes *ds.RingBuffer[time.Duration]
}

func NewPlayerData() *PlayerData {
	return &PlayerData{
		id: ecs.InvalidEntity,
		inputBuffer: make([]InputBufferItem, 0),
		roundTripTimes: ds.NewRingBuffer[time.Duration](100), // TODO - configurable
	}
}

func (p *PlayerData) Id() ecs.Id {
	p.mu.RLock()
	ret := p.id
	p.mu.RUnlock()
	return ret
}

func (p *PlayerData) SetId(id ecs.Id) {
	p.mu.Lock()
	p.id = id
	p.mu.Unlock()
}

// func (p *PlayerData) Tick() uint16 {
// 	p.mu.RLock()
// 	ret := p.tick
// 	p.mu.RUnlock()
// 	return ret
// }

func (p *PlayerData) SetTicks(serverTick, serverUpdatePlayerTick uint16) {
	// fmt.Println("SetTicks: ", serverTick, serverUpdatePlayerTick, time.Now())

	p.mu.Lock()
	defer p.mu.Unlock()

	// Set the last server tick we've received
	p.serverTick = serverTick

	// Cut off every player input tick that the server hasn't processed
	cut := int(p.playerTick - serverUpdatePlayerTick)
	// fmt.Println("InputBuffer", p.serverTick, p.playerTick, serverUpdatePlayerTick, len(p.inputBuffer))
	for i := 0; i < len(p.inputBuffer)-cut; i++ {
		p.roundTripTimes.Add(time.Since(p.inputBuffer[i].Time))
	}

	if cut >= 0 && cut <= len(p.inputBuffer) {
		// TODO - it'd be more efficient to use a queue
		copy(p.inputBuffer, p.inputBuffer[len(p.inputBuffer)-cut:])
		p.inputBuffer = p.inputBuffer[:cut]
		// fmt.Println("Copied", n, len(p.inputBuffer))
	} else {
		// fmt.Println("OOB")
	}
}

// Returns the player tick that this input is associated with
func (p *PlayerData) AppendInputTick(input mmo.Input) uint16 {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.playerTick = (p.playerTick + 1) % math.MaxUint16
	p.inputBuffer = append(p.inputBuffer, InputBufferItem{
		Input: input,
		Time: time.Now(),
	})
	return p.playerTick
}

func (p *PlayerData) GetInputBuffer() []InputBufferItem {
	return p.inputBuffer
}

func (p *PlayerData) RoundTripTimes() []time.Duration {
	return p.roundTripTimes.Buffer()
}

// Returns the message as sent to the server
// TODO - if the player sends another message fast enough, it could blank out their first message
// func (p *PlayerData) SendMessage(msg string) string {
// 	msg = serdes.FilterChat(msg)
// 	p.lastMessage = msg
// 	return msg
// }

// // Returns the last message and clears the last message buffer, returns nil if no new message
// func (p *PlayerData) GetLastMessage() *game.ChatMessage {
// 	if p.lastMessage == "" {
// 		return nil
// 	}

// 	msg := p.lastMessage
// 	p.lastMessage = ""
// 	return &game.ChatMessage{
// 		// Username: nil, // TODO - return username?
// 		Message: msg,
// 	}
// }
