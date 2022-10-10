.PHONY: all client proxy server

all: client proxy server
	mkdir -p build

server:
	CGO_ENABLED=0 go build -o build/server ./server/

proxy: build/keygen
	CGO_ENABLED=0 go build -o build/proxy ./proxy/

build/keygen:
	openssl req -newkey rsa:2048 -x509 -nodes -days 365 -keyout build/privkey.pem -out build/cert.pem -config <(cat /etc/ssl/openssl.cnf <(printf '[SAN]\nsubjectAltName=DNS:localhost,IP:127.0.0.1'))
#	openssl req -newkey rsa:2048 -x509 -nodes -days 365 -keyout build/nginx-proxy.key -out build/nginx-proxy.crt -config <(cat /etc/ssl/openssl.cnf <(printf '[SAN]\nsubjectAltName=DNS:hostname,IP:127.0.0.1'))
	openssl dhparam -out build/dhparam.pem 2048
	touch build/keygen # This is just to make sure that it doesn't re-execute

client:
	cd client && $(MAKE) wasm
	cp client/mmo.wasm build/mmo.wasm
	cp client/index.html build/
	cp "$(shell go env GOROOT)/misc/wasm/wasm_exec.js" build/

lag-international:
	tc qdisc add dev lo root netem delay 200ms 40ms 25% loss 15.3% 25% duplicate 1% corrupt 0.1% reorder 5% 50%
lag-local:
	tc qdisc add dev lo root netem delay 25ms 5ms 25% loss 1% 5% duplicate 1% corrupt 0.1% reorder 1% 50%
lag-good:
	tc qdisc add dev lo root netem delay 25ms 5ms 25%
lag-remove:
	tc qdisc del dev lo root netem
