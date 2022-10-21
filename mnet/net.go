package mnet

import (
	"fmt"
	"errors"
	"time"
	"net"
	"net/url"
	// "net/http"
	// "crypto/tls"
	"sync"
	"sync/atomic"
	"context"

	"nhooyr.io/websocket"

	"github.com/rs/zerolog/log"

	"github.com/unitoftime/mmo/serdes"
)

// TODO - Ensure sent messages remain under this
// Calculation: 1460 Byte = 1500 Byte - 20 Byte IP Header - 20 Byte TCP Header
const MaxMsgSize = 1460 // bytes
const MaxRecvMsgSize = 4 * 1024 // 4 KB // TODO - this is arbitrary

var ErrSerdes = errors.New("serdes errror")
var ErrNetwork = errors.New("network error")

// Continually attempts to reconnect to the proxy if disconnected. If connected, receives data and sends over the networkChannel
//func ReconnectLoop(world *ecs.World, sock *Socket, playerId *ecs.Id, updateChan chan serdes.WorldUpdate) {
func ReconnectLoop(sock *Socket, handler func(*Socket) error) {
	for {
		if sock.Closed.Load() { break } // Exit if the ClientConn has been closed

		err := sock.Dial()
		if err != nil {
			log.Warn().Err(err).Msg("Client Websocket Dial Failed")
			time.Sleep(5 * time.Second) // TODO - Probably want some random value so everyone isn't reconnecting simultaneously
			continue
		}

		// Start the handler
		err = handler(sock)
		if err != nil {
			log.Warn().Err(err).Msg("ReconnectLoop handler finished")

			// TODO - Is this a good idea?
			// Try to close the connection one last time
			sock.conn.Close()

			// Set connected to false, because we just closed it
			sock.Connected.Store(false)
		}
		log.Print("Looping!")
	}

	// Final attempt to cleanup the connection
	sock.Connected.Store(false)
	sock.conn.Close()
	log.Print("Exiting ClientConn.ReconnectLoop")
}

// This is a wrapper for the client websocket connection
type Socket struct {
	url string // The URL to connect to
	scheme string // The scheme of the parsed URL
	host string // The host of the parsed URL
	encoder *serdes.Serdes // The encoder to use for serialization
	conn net.Conn          // The underlying network connection to send and receive on

	// Note: sendMut I think is needed now that I'm using goframe
	sendMut sync.Mutex // The mutex for multiple threads writing at the same time
	recvMut sync.Mutex // The mutex for multiple threads reading at the same time
	recvBuf []byte     // The buffer that reads are buffered into

	Closed atomic.Bool // Used to indicate that the user has requested to close this ClientConn
	Connected atomic.Bool // Used to indicate that the underlying connection is still active
}

// TODO - Combine NewSocket and NewConnectedSocket
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
		recvBuf: make([]byte, MaxRecvMsgSize),
	}
	return &sock, nil
}

func NewConnectedSocket(conn net.Conn) *Socket {
	sock := Socket{
		// Create a Framed connection and set it to our connection
		// conn: NewFrameConn(conn),
		conn: conn, // TODO - need to frame when doing TCP and not frame when doing WS. How to handle? Maybe move server abstraction over or something?
		encoder: serdes.New(),
		recvBuf: make([]byte, MaxRecvMsgSize),
	}
	return &sock
}

func (s *Socket) Dial() error {
	log.Print("Dialing", s.url)
	// Handle websockets
	if s.scheme == "ws" || s.scheme == "wss" {
		// // TODO - Do this for local testing (Right now I'm doing insecure skip verify)
		// Ref: https://github.com/jcbsmpsn/golang-https-example
		// cert, err := os.ReadFile("cert.pem")
		// if err != nil {
		// 	panic(err)
		// }
		// caCertPool := x509.NewCertPool()
		// caCertPool.AppendCertsFromPEM(caCert)

		// ctx := context.Background()
		// wsConn, _, err := websocket.Dial(ctx, s.url, nil)
		ctx := context.Background()
		wsConn, err := dialWs(ctx, s.url)


		// log.Println("Connection Response:", resp)
		if err != nil { return err }

		// Note: This connection is automagically framed by websockets
		s.conn = websocket.NetConn(ctx, wsConn, websocket.MessageBinary)
		s.Connected.Store(true)
		return nil
	} else if s.scheme == "tcp" {
		conn, err := net.Dial("tcp", s.host)
		if err != nil { return err }

		// Create a Framed connection and set it to our connection
		s.conn = NewFrameConn(conn)
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
	if s.conn == nil {
		return fmt.Errorf("Socket Closed")
	}

	ser, err := s.encoder.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize")
	}

	s.sendMut.Lock()
	defer s.sendMut.Unlock()

	// log.Println("ClientSendUpdate:", len(ser))
	_, err = s.conn.Write(ser)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrNetwork, err)
		return err
	}
	return nil
}
// Reads the next message (blocking) on the connection
func (s *Socket) Recv() (any, error) {
	if s.conn == nil {
		return nil, fmt.Errorf("Socket Closed")
	}

	s.recvMut.Lock()
	defer s.recvMut.Unlock()

	n, err := s.conn.Read(s.recvBuf)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to receive")
		err = fmt.Errorf("%w: %s", ErrNetwork, err)
		return nil, err
	}
	if n <= 0 { return nil, nil } // There was no message, and no error (likely a keepalive)

	// log.Print("Read bytes: ", n)
//	log.Print("Data: ", s.recvBuf)
	// log.Print("Data: ", s.recvBuf[:n])

	// Note: slice off based on how many bytes we read
	msg, err := s.encoder.Unmarshal(s.recvBuf[:n])
	if err != nil {
		log.Warn().Err(err).Msg("Failed to unmarshal")
		err = fmt.Errorf("%w: %s", ErrSerdes, err)
		return nil, err
	}
	return msg, nil
}
