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
    environment:
      - ALERTMANAGER_ADDRESS=alertmanager
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - 8080:8080
    deploy:
      placement:
        constraints: [node.role == manager]
  web:
    image: alpine:3.6
    deploy:
      replicas: 3
      labels:
        com.df.scaleMin: "2"
        com.df.scaleMax: "4"
    command: sleep 10000000
  alertmanager:
    image: prom/alertmanager:v0.8.0
    ports:
      - 9093:9093

```

The `web` service represents a long running process that can be scaled. The labels `com.df.scaleMin`
and `com.df.scaleMax` represents the minimum and maximum number of replicas for the `web` service.

## Example

Deploying `script/docker-compose-example.yml` as a stack:
```bash
$ docker stack deploy -c scripts/docker-compose-example.yml example
```
Following the naming convention of `docker stack deploy`, this will create three services `example_scaler`, `example_web`, `example_alertmanager`. Port `8080` exposes the `example_scaler` service and port `9093` exposes `example_alertmanager` to your local machine. To scale `example_web` up by one replica send the following request:
```bash
$ curl -X POST localhost:8080/scale?service=example_web&delta=1
```
This will also send an alert to the alertmanager, you can query the alertmanager by installing [amtool](https://github.com/prometheus/alertmanager) and running:
```
$ amtool --alertmanager.url http://localhost:9093 alert
```
This will list the alerts received by the alertmanager:
```
Alertname      Starts At                Summary
scale_service  2017-09-25 16:44:12 UTC  Scaling example_web to 4 replicas
```
To scale `example_web` down by one, send the following request:
```bash
$ curl -X POST localhost:8080/scale?service=example_web&delta=-1
```
Running the `amtool` query again will display:
```
Alertname      Starts At                Summary
scale_service  2017-09-25 16:55:01 UTC  Scaling example_web to 3 replicas
```
If you wish to display all the information in an alert run:
```bash
$ amtool --alertmanager.url http://localhost:9093 -o extended alert
```

### AWS Integration

Create secret for AWS access
```
echo 'export AWS_ACCESS_KEY_ID=xxxx
export AWS_SECRET_ACCESS_KEY=xxxx
export AWS_DEFAULT_REGION=us-east-1
' | docker secret create aws -
```

Deploying `scripts/docker-compose-aws.yml` as a stack:
```bash
$ docker stack deploy -c scripts/docker-compose-aws.yml aws
```

Send request to scale manager node:
```bash
$ curl -X POST localhost:8080/scale?nodesOn=aws&
```
