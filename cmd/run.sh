#!/bin/bash

trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT

gotip run ./server &
sleep 1
gotip run ./proxy &
sleep 1
cd client && gotip run . -cpuprofile cpu.prof -memprofile mem.prof
