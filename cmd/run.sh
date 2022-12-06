#!/bin/bash

trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT
set -e

go run ./server &
sleep 2
go run ./proxy &
#sleep 2
#go run ./client --skip &
go run ./client --skip
