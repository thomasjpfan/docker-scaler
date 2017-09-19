#!/bin/sh

docker run --rm \
-v "/var/run/docker.sock:/var/run/docker.sock" \
-v "$(pwd)":/go/src/github.com/thomasjpfan/docker-scaler \
golang:1.9.0-alpine3.6 \
go test ./... --run UnitTest -v
