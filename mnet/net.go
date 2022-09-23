package mnet

import (
	"fmt"
	"log"
	"errors"
	"time"
	"net"
	"net/url"
	"sync/atomic"
	"context"

	"nhooyr.io/websocket"

	"github.com/unitoftime/mmo/serdes"
	// "github.com/unitoftime/ecs"
)

var ErrSerdes = errors.New("serdes errror")
var ErrNetwork = errors.New("network error")

// Continually attempts to reconnect to the proxy if disconnected. If connected, receives data and sends over the networkChannel
//func ReconnectLoop(world *ecs.World, sock *Socket, playerId *ecs.Id, updateChan chan serdes.WorldUpdate) {
func ReconnectLoop(sock *Socket, handler func(*Socket) error) {
	for {
		if sock.Closed.Load() { break } // Exit if the ClientConn has been closed

		err := sock.Dial()
		if err != nil {
			log.Println("Client Websocket Dial Failed:", err)
			time.Sleep(2 * time.Second) // TODO - reconfigure this before launch. Probably want some random value so everyone isn't reconnecting simultaneously
			continue
		}

		// Start the handler
		// err = ClientReceive(world, c, playerId, updateChan)
		err = handler(sock)
		if err != nil {
			log.Println("ClientReceive Exited:", err)

			// TODO - Is this a good idea?
			// Try to close the connection one last time
			sock.conn.Close()

			// Set connected to false, because we just closed it
			sock.Connected.Store(false)
		}
		log.Println("Looping!")
	}

	// Final attempt to cleanup the connection
	sock.Connected.Store(false)
	sock.conn.Close()
	log.Println("Exiting ClientConn.ReconnectLoop")
}

// This is a wrapper for the client websocket connection
type Socket struct {
	url string // The URL to connect to
	scheme string // The scheme of the parsed URL
	host string // The host of the parsed URL
	encoder *serdes.Serdes // The encoder to use for serialization
	conn net.Conn          // The underlying network connection to send and receive on

	Closed atomic.Bool // Used to indicate that the user has requested to close this ClientConn
	Connected atomic.Bool // Used to indicate that the underlying connection is still active
}

func NewSocket(network string) (*Socket, error) {
	u, err := url.Parse(network)
	if err != nil {
		return nil, err
	}

	sock := Socket{
		scheme: u.Scheme,
		host: u.Host,
		url: network,
		encoder: serdes.New(),
	}
	return &sock, nil
}

func NewConnectedSocket(conn net.Conn) *Socket {
	sock := Socket{
		conn: conn,
		encoder: serdes.New(),
	}
	return &sock
}

func (s *Socket) Dial() error {
	// Handle websockets
	if s.scheme == "ws" {
		log.Println("Dialing", s.url)
		ctx := context.Background()
		wsConn, _, err := websocket.Dial(ctx, s.url, nil)
		// log.Println("Connection Response:", resp)
		if err != nil { return err }

		s.conn = websocket.NetConn(ctx, wsConn, websocket.MessageBinary)
		s.Connected.Store(true)
		return nil
	} else if s.scheme == "tcp" {
		conn, err := net.Dial("tcp", s.host)
		if err != nil { return err }

		s.conn = conn
		s.Connected.Store(true)
		return nil
	}

	return fmt.Errorf("Failed to Dial, unknown ClientConn")
}

func (s *Socket) Close() {
	s.Connected.Store(false)
	s.Closed.Store(true)
	if s.conn != nil {
		s.conn.Close()
	}
}

// Sends the message through the connection
func (s *Socket) Send(msg any) error {
	ser, err := s.encoder.Marshal(msg)
	if err != nil {
		log.Println("Failed to serialize", err)
	}

	// log.Println("ClientSendUpdate:", len(ser))
	_, err = s.conn.Write(ser)
	if err != nil {
		log.Println("Error Sending:", err)
		err = fmt.Errorf("%w: %s", ErrNetwork, err)
		return err
	}
	return nil
}

// Reads the next message (blocking) on the connection
// TODO - Thread safety?
func (s *Socket) Recv() (any, error) {
	if s.conn == nil {
		return nil, fmt.Errorf("Socket Closed")
	}
	// TODO optimize this buffer creation
	const MaxMsgSize int = 4 * 1024
	dat := make([]byte, MaxMsgSize)

	n, err := s.conn.Read(dat)
	if err != nil {
		log.Println("Read Error:", err)
		err = fmt.Errorf("%w: %s", ErrNetwork, err)
		return nil, err
	}
	if n <= 0 { return nil, nil } // There was no message, and no error (likely a keepalive)

	log.Println("Read bytes", n)

	// Note: slice off based on how many bytes we read
	msg, err := s.encoder.Unmarshal(dat[:n])
	if err != nil {
		log.Println("Failed to unmarshal:", err)
		err = fmt.Errorf("%w: %s", ErrSerdes, err)
		return nil, err
	}
	return msg, nil
}
