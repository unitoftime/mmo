#!/bin/bash

trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT

cd client
for i in {0..10}
do
    echo "NewClient" ${i}
    go run . -skip &
done

go run . -skip

