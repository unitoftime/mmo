package mmo

import (
	"fmt"
	"log"
	"errors"
	"net"
	"sync/atomic"
	"context"

	"nhooyr.io/websocket"

	"github.com/unitoftime/mmo/serdes"
)

var ErrSerdes = errors.New("serdes errror")
var ErrNetwork = errors.New("network error")

// This is a wrapper for the client websocket connection
type ClientConn struct {
	url string // The URL to connect to
	encoder *serdes.Serdes // The encoder to use for serialization
	conn net.Conn          // The underlying network connection to send and receive on

	updateChan chan serdes.WorldUpdate // The channel to forward world update messages

	Closed atomic.Bool // Used to indicate that the user has requested to close this ClientConn
	Connected atomic.Bool // Used to indicate that the underlying connection is still active
}
func NewClientConn(url string, networkChannel chan serdes.WorldUpdate) *ClientConn {
	clientConn := ClientConn{
		url: url,
		encoder: serdes.New(),
		updateChan: networkChannel,
	}
	return &clientConn
}

func (c *ClientConn) Dial() error {
	log.Println("Dialing", c.url)
	ctx := context.Background()
	wsConn, _, err := websocket.Dial(ctx, c.url, nil)
	// log.Println("Connection Response:", resp)
	if err != nil { return err }

	c.conn = websocket.NetConn(ctx, wsConn, websocket.MessageBinary)
	c.Connected.Store(true)
	return nil
}

func (c *ClientConn) Close() {
	c.Connected.Store(false)
	c.Closed.Store(true)
	c.conn.Close()
}

// Sends the message through the connection
func (c *ClientConn) Send(msg any) error {
	ser, err := c.encoder.Marshal(msg)
	if err != nil {
		log.Println("Failed to serialize", err)
	}

	// log.Println("ClientSendUpdate:", len(ser))
	_, err = c.conn.Write(ser)
	if err != nil {
		log.Println("Error Sending:", err)
		err = fmt.Errorf("%w: %s", ErrNetwork, err)
		return err
	}
	return nil
}

// Reads the next message (blocking) on the connection
// TODO - Thread safety?
func (c *ClientConn) Recv() (any, error) {
	// TODO optimize this buffer creation
	const MaxMsgSize int = 4 * 1024
	dat := make([]byte, MaxMsgSize)

	n, err := c.conn.Read(dat)
	if err != nil {
		log.Println("Read Error:", err)
		err = fmt.Errorf("%w: %s", ErrNetwork, err)
		return nil, err
	}
	if n <= 0 { return nil, nil } // There was no message, and no error (likely a keepalive)

	log.Println("Read bytes", n)

	// Note: slice off based on how many bytes we read
	msg, err := c.encoder.Unmarshal(dat[:n])
	if err != nil {
		log.Println("Failed to unmarshal:", err)
		err = fmt.Errorf("%w: %s", ErrSerdes, err)
		return nil, err
	}
	return msg, nil
}
