package mnet

import (
	"net"
	"time"
	"encoding/binary"

	// TODO - Implement my own framing?
	"github.com/smallnest/goframe"
)

type FrameConn struct {
	frameConn goframe.FrameConn
	conn net.Conn
}

func NewFrameConn(conn net.Conn) *FrameConn {
	encCfg := goframe.EncoderConfig{
		ByteOrder: binary.BigEndian,
		LengthFieldLength: 2, // Note: 2 byte length, maximum 64k
	}
	decCfg := goframe.DecoderConfig{
		ByteOrder: binary.BigEndian,
		LengthFieldLength: 2, // Note: 2 byte length, maximum 64k
		InitialBytesToStrip: 2, // Note: Strip out the first two bytes (ie the lenghtField)
	}

	return &FrameConn{
		frameConn: goframe.NewLengthFieldBasedFrameConn(encCfg, decCfg, conn),
		conn: conn,
	}
}

func (f *FrameConn) Read(b []byte) (int, error) {
	// TODO - hack because goframe creates a buffer for reads. Unless I want to allocate on every read I need to eventually have it pass in else it might waste memory. I'm just going to copy. Or actually it might be more efficient to have the connection manage the buffers then just return those with the expectation that they'll be finished being used before the next read (or something)
	tmpBuf, err := f.frameConn.ReadFrame()
	if err != nil {
		return 0, err // TODO - Assuming I return 0 length here? I guess I'm not sure. Maybe we have read some values?
	}

	length := copy(b, tmpBuf)
	return length, nil
}

func (f *FrameConn) Write(dat []byte) (int, error) {
	return len(dat), f.frameConn.WriteFrame(dat)
}

func (f *FrameConn) Close() error {
	return f.frameConn.Close()
}

func (f *FrameConn) LocalAddr() net.Addr {
	return f.conn.LocalAddr()
}

func (f *FrameConn) RemoteAddr() net.Addr {
	return f.conn.RemoteAddr()
}

func (f *FrameConn) SetDeadline(t time.Time) error {
	return f.conn.SetDeadline(t)
}

func (f *FrameConn) SetReadDeadline(t time.Time) error {
	return f.conn.SetReadDeadline(t)
}

func (f *FrameConn) SetWriteDeadline(t time.Time) error {
	return f.conn.SetWriteDeadline(t)
}
