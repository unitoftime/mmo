#!/bin/bash

trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT
set -e

go run ./server &
go run ./proxy &
go run ./client --skip
