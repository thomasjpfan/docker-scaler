#!/bin/sh

docker run --rm \
-v "$(pwd)":/go/src/github.com/thomasjpfan/docker-scaler \
--network test_scaling \
-v "/var/run/docker.sock:/var/run/docker.sock" \
-e "SCALER_IP=test_scaler" \
-e "TARGET_SERVICE=test_web" \
golang:1.9.0-alpine3.6 \
go test github.com/thomasjpfan/docker-scaler/integration -v
