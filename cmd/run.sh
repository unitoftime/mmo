#!/bin/bash

trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT
go run ./server. &
cd client && go run . &
cd client && go run .
