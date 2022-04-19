// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package msg

import (
	flatbuffers "github.com/google/flatbuffers/go"
)

type Message struct {
	_tab flatbuffers.Table
}

func GetRootAsMessage(buf []byte, offset flatbuffers.UOffsetT) *Message {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &Message{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsMessage(buf []byte, offset flatbuffers.UOffsetT) *Message {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &Message{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *Message) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *Message) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *Message) PayloadType() Payload {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return Payload(rcv._tab.GetByte(o + rcv._tab.Pos))
	}
	return 0
}

func (rcv *Message) MutatePayloadType(n Payload) bool {
	return rcv._tab.MutateByteSlot(4, byte(n))
}

func (rcv *Message) Payload(obj *flatbuffers.Table) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		rcv._tab.Union(obj, o)
		return true
	}
	return false
}

func MessageStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}
func MessageAddPayloadType(builder *flatbuffers.Builder, payloadType Payload) {
	builder.PrependByteSlot(0, byte(payloadType), 0)
}
func MessageAddPayload(builder *flatbuffers.Builder, payload flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(payload), 0)
}
func MessageEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}