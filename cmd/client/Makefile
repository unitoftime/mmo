linux:
	go build -ldflags "-s" -v .

wasm:
#	cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" .
	GOOS=js GOARCH=wasm go build -ldflags "-s" -o mmo.wasm

windows:
	GOOS=windows GOARCH=386 CGO_ENABLED=1 CXX=i686-w64-mingw32-g++ CC=i686-w64-mingw32-gcc go build -ldflags "-s" -v .
