#!/bin/bash

# Wait for scaler to become active
counter=0
until [ "$(docker service ls -f 'name=test_scaler' | \
        tail -n1 | awk '{ print $4 }' | cut -d "/" -f1)" == "1" ]; do
    echo "Waiting for test_scaler to start"
    sleep 1
    counter=$((counter+1))
    if [ "$counter" == "60" ]; then
        echo "Waited one minute! test_scaler did not start"
        exit 1
    fi
done


docker run --rm \
-v "$(pwd)":/go/src/github.com/thomasjpfan/docker-scaler \
--network test_scaling \
--network test_alert \
-v "/var/run/docker.sock:/var/run/docker.sock" \
-e "SCALER_IP=scaler" \
-e "TARGET_SERVICE=test_web1" \
-e "FALSE_RESCHEDULE_SERVICE=test_web2" \
-e "ALERTMANAGER_ADDRESS=http://alertmanager:9093" \
golang:1.9.0-alpine3.6 \
go test github.com/thomasjpfan/docker-scaler/integration -v
