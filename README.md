# Docker Scaler

[![Build Status](https://travis-ci.org/thomasjpfan/docker-scaler.svg?branch=master)](https://travis-ci.org/thomasjpfan/docker-scaler)

Microservice providing a REST API that scales services in Docker Swarm.

Please visit the [documentation](https://thomasjpfan.github.io/docker-scaler/) for configuration and tutorials.

## Usage

Consider the services defined by the following compose spec:

```yml
version: "3.3"

services:
  scaler:
    image: thomasjpfan/docker-scaler:latest
    environment:
      - ALERTMANAGER_ADDRESS=http://alertmanager:9093
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
        com.df.scaleMin: 2
        com.df.scaleMax: 4
        com.df.scaleDownBy: 1
        com.df.scaleUpBy: 2
    command: sleep 10000000
  alertmanager:
    image: prom/alertmanager:v0.8.0
    ports:
      - 9093:9093

```

The `web` service represents a long running process that can be scaled. The labels `com.df.scaleMin`
and `com.df.scaleMax` represents the minimum and maximum number of replicas for the `web` service.

## Example

Deploying `script/docker-scaler-readme.yml` as a stack:
```bash
$ docker stack deploy -c stacks/docker-scaler-readme.yml example
```
Following the naming convention of `docker stack deploy`, this will create three services `example_scaler`, `example_web`, `example_alertmanager`. Port `8080` exposes the `example_scaler` service and port `9093` exposes `example_alertmanager` to your local machine. To scale `example_web` up, send the following request:
```bash
$ curl -X POST localhost:8080/v1/scale-service -X POST -d \
'{"groupLabels": {"scale": "up", "service": "example_web"}}'
```
`example_web` will scale up by `com.df.scaleUpBy` label, in this case the number of replicas will go from 3 to 5. An alert to the alertmanager, you can query the alertmanager by installing [amtool](https://github.com/prometheus/alertmanager) and running:
```
$ amtool --alertmanager.url http://localhost:9093 alert
```
This will list the alerts received by the alertmanager:
```
Alertname      Starts At                Summary
scale_service  2017-11-18 17:46:39 UTC  Scaling example_web from 3 to 5 replicas
```
To scale `example_web` down by one, send the following request:
```bash
$ curl -X POST localhost:8080/v1/scale-service -X POST -d \
'{"groupLabels": {"scale": "down", "service": "example_web"}}'
```
`example_web` will scale down by `com.df.scaleDownBy` label, in this case the number of replicas will go from 5 to 4. Running the `amtool` query again will display:
```
Alertname      Starts At                Summary
scale_service  2017-11-18 17:55:23 UTC  Scaling example_web from 5 to 4 replicas
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
' | docker secret create aws -
```
In `scripts/docker-scaler-aws.yml`, overwrite `AWS_MANAGER_ASG` and `AWS_WORKER_ASG`
with your autoscaling group names. Then deploy it as a stack:
```bash
$ docker stack deploy -c scripts/docker-scaler-aws.yml aws
```
To send request to scale up worker node by 1:
```bash
$ curl -X POST [PUBLIC_DNS]:8080/v1/scale-nodes?by=1&type=worker -d \
'{"groupLabels": {"scale": "up"}}'
```
Or scaling up a manager node by 2:
```bash
$ curl -X POST [PUBLIC_DNS]:8080/v1/scale-nodes?by=2&type=manager -d \
'{"groupLabels": {"scale": "up"}}'
```
