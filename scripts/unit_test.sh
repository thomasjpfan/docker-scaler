#!/bin/sh

docker run --rm \
-v "$(pwd)":/go/src/github.com/thomasjpfan/docker-scaler \
golang:1.9.0-alpine3.6 \
go test ./... --run UnitTest -v
