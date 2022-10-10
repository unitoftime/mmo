// +build !js

package mnet

import (
	"context"
	"net/http"
	"crypto/tls"

	"nhooyr.io/websocket"
)

func dialWs(ctx context.Context, url string) (*websocket.Conn, error) {
	wsConn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	})
	return wsConn, err
}
