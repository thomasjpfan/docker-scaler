# Docker Scaler

[![Build Status](https://travis-ci.org/thomasjpfan/docker-scaler.svg?branch=master)](https://travis-ci.org/thomasjpfan/docker-scaler)

Microservice providing a REST API that scales services in Docker Swarm.

## Usage

Consider the services defined by the following compose spec:

```yml
version: "3.3"

services:
  scaler:
    image: thomasjpfan/docker-scaler:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - 8080:8080
    deploy:
      placement:
        constraints: [node.role == manager]
  web:
    image: alpine:3.6
    labels:
      com.df.scaleMin: "2"
      com.df.scaleMax: "4"
    deploy:
      replicas: 3
    command: sleep 10000000

```

The `web` service represents a long running process that can be scaled. The labels `com.df.scaleMin`
and `com.df.scaleMax` represents the minimum and maximum number of replicas for the `web` service.

## Example

Deploying `script/docker-compose-example.yml` as a stack:
```
docker stack deploy -c scripts/docker-compose-example.yml example
```
Follwing the naming convention of `docker stack deploy`, this will create two services `example_scaler` and `example_web`. Running this on your local machine will expose `localhost:8080` as the endpoint to the `example_scaler` service. To scale `example_web` up by one replica send the following request:
```
curl -X POST localhost:8080/scale\?service=example_web\&delta=1
```
To scale `example_web` by one replica down send the following request:
```
curl -X POST localhost:8080/scale\?service=example_web\&delta=-1
```

