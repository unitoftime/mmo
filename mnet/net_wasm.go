// +build js,wasm

package mnet

import (
	"context"

	"nhooyr.io/websocket"
)

func dialWs(ctx context.Context, url string) (*websocket.Conn, error) {
	wsConn, _, err := websocket.Dial(ctx, url, nil)
	return wsConn, err
}
