# linux:
# 	go build -ldflags "-s" -v .

wasm:
#	cp "$(gotip env GOROOT)/misc/wasm/wasm_exec.js" .
	GOOS=js GOARCH=wasm gotip build -ldflags "-s" -o main.wasm

windows:
	env GOOS=windows GOARCH=386 CGO_ENABLED=1 CXX=i686-w64-mingw32-g++ CC=i686-w64-mingw32-gcc go build -ldflags "-s" -v .
