#!/bin/bash

trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT

go run ./server &
go run ./proxy &
go run ./client
