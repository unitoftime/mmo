#!/bin/bash

trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT
set -e

go run ./server &
sleep 1
go run ./proxy &
#go run ./client --skip &
go run ./client --skip
