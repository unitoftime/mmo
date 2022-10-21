package main

import (
	"github.com/unitoftime/mmo/app/proxy"
)

func main() {
	proxy.Main(proxy.Config{
		ServerUri: "tcp://127.0.0.1:9000",
		Test: true,
		CertFile: "./build/cert.pem",
		KeyFile: "./build/privkey.pem",
	})
}
