# Auto-Scaling Services With Instrumented Metrics

*Docker Scaler* provides an alternative to using Jenkins for service scaling shown in Docker Flow Monitor's [auto-scaling tutorial](http://monitor.dockerflow.com/auto-scaling/). In this tutorial, we will construct a system that will scale a service based on response time. Here is an overview of the triggered events in our self-adapting system:

1. The [go-demo](https://github.com/vfarcic/go-demo) service response times becomes high.
2. [Docker Flow Monitor](http://monitor.dockerflow.com/) is querying the services' metrics, notices the high response times, and alerts the [Alertmanager](https://prometheus.io/docs/alerting/alertmanager/).
3. The Alertmanager is configured to forward the alert to *Docker Scaler*.
4. *Docker Scaler* scales the service up.

This tutorial assumes you have Docker Machine version v0.8+ that includes Docker Engine v1.12+.

!!! info
	If you are a Windows user, please run all the examples from *Git Bash* (installed through *Docker for Windows*). Also, make sure that your Git client is configured to check out the code *AS-IS*. Otherwise, Windows might change carriage returns to the Windows format.

We will be using *Slack* webhooks to notify us. Create a *Slack* channel and setup a webhook by consulting *Slack's* [Incoming Webhook](https://get.slack.help/hc/en-us/articles/115005265063-Incoming-WebHooks-for-Slack) page. After obtaining a webhook URL set it as an environment variable:

```bash
export SLACK_WEBHOOK_URL=[...]
```

## Setting Up A Cluster

!!! info
    Feel free to skip this section if you already have a Swarm cluster that can be used for this tutorial

We create a Swarm cluster consisting of three nodes created with Docker Machine.

```bash
git clone https://github.com/thomasjpfan/docker-scaler.git

cd docker-scaler

./scripts/ds-swarm.sh

eval $(docker-machine env swarm-1)
```

The repo contains all the scripts and stack files needed throughout this tutorial. Next, we executed `ds-swarm.sh` creating the cluster with `docker-machine`. Finally, we used the `eval` command to tell our local Docker client to use the remote Docker Engine `swarm-1`.

## Deploying Docker Flow Proxy (DFP) and Docker Flow Swarm Listener (DFSL)

For convenience, we will use *Docker Flow Proxy* and *Docker Flow Swarm Listener* to get a single access point to the cluster.

```bash
docker network create -d overlay proxy

docker stack deploy \
    -c stacks/docker-flow-proxy-mem.yml \
    proxy
```

Please visit [proxy.dockerflow.com](http://proxy.dockerflow.com) and [swarmlistener.dockerflow.com](http://swarmlistener.dockerflow.com/) for details on the *Docker Flow* stack.

## Deploying Docker Scaler

We can now deploy the *Docker Scaler* stack:

```bash
docker network create -d overlay scaler

docker stack deploy \
    -c stacks/docker-scaler-service-scale-tutorial.yml \
    scaler
```

This stack defines a single *Docker Scaler* service:

```yaml
...
  services:
    scaler:
      image: thomasjpfan/docker-scaler
      environment:
        - ALERTMANAGER_ADDRESS=http://alertmanager:9093
        - SERVER_PREFIX=/scaler
      volumes:
        - /var/run/docker.sock:/var/run/docker.sock
      networks:
        - scaler
      deploy:
        replicas: 1
        labels:
          - com.df.notify=true
          - com.df.distribute=true
          - com.df.servicePath=/scaler
          - com.df.port=8080
        placement:
            constraints: [node.role == manager]
...
```

This definition constraints *Docker Scaler* to run on manager nodes and gives it access to the Docker socket, so that it can scale services in the cluster. The label `com.df.servicePath=/scaler` and environement variable `SERVER_PREFIX=/scaler` allows us to interact with the `scaler` service.

## Deploying Docker Flow Monitor and Alertmanager

The next stack defines the *Docker Flow Monitor* and *Alertmanager* services. Before we deploy the stack, we defined our *Alertmanager* configuration as a Docker secret:

```bash
echo "global:
  slack_api_url: '$SLACK_WEBHOOK_URL'
route:
  receiver: 'slack'
  group_by: [service, scale, type]
  group_interval: 5m
  repeat_interval: 5m
  routes:
  - match_re:
      scale: up|down
      type: service
    receiver: 'scale'
  - match:
      alertname: scale_service
    group_by: [alertname, service]
    repeat_interval: 10s
    group_interval: 1s
    group_wait: 0s
    receiver: 'slack-scaler'

receivers:
  - name: 'slack'
    slack_configs:
      - send_resolved: true
        title: '[{{ .Status | toUpper }}] {{ .GroupLabels.service }} service is in danger!'
        title_link: 'http://$(docker-machine ip swarm-1)/monitor/alerts'
        text: '{{ .CommonAnnotations.summary }}'
  - name: 'slack-scaler'
    slack_configs:
      - title: '{{ .GroupLabels.alertname }}: {{ .CommonAnnotations.request }}'
        color: '{{ if eq .CommonLabels.status \"error\" }}danger{{ else }}good{{ end }}'
        title_link: 'http://$(docker-machine ip swarm-1)/monitor/alerts'
        text: '{{ .CommonAnnotations.summary }}'
  - name: 'scale'
    webhook_configs:
      - send_resolved: false
        url: 'http://scaler:8080/v1/scale-service'
" | docker secret create alert_manager_config -
```

This configuration groups alerts by their `service`, `scale`, and `type` labels. The `routes` section defines a `match_re` entry, that directs scale alerts to the `scale` reciever. Another route is configured to direct alerts from the `scaler` service to the `slack-scaler` receiver. Now we deploy the monitor stack:

```bash
docker network create -d overlay monitor

DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-slack.yml \
    monitor
```

The `alert-manager` service is configured to read the `alert_manager_config` secret in the stack definition as follows:

```yaml
...
  alert-manager:
    image: prom/alertmanager:v0.14.0
    networks:
      - monitor
      - scaler
    secrets:
      - alert_manager_config
    command: --config.file=/run/secrets/alert_manager_config --storage.path=/alertmanager
...
```

With access to the `scaler` network, `alert-manager` can send scaling requests to the `scaler` service. For information about the Docker Flow Monitor stack can be found in its [documentation](http://monitor.dockerflow.com).

Let us confirm that the `monitor` stack is up and running:

```bash
docker stack ps monitor
```

Please wait a few moments for all the replicas to have the status `running`. After the `monitor` stack is up and running.

## Manually Scaling Services

For this section, we deploy a simple sleeping service:

```bash
docker service create -d --replicas 1 \
  --name demo \
  -l com.df.scaleUpBy=3 \
  -l com.df.scaleDownBy=2 \
  -l com.df.scaleMin=1 \
  -l com.df.scaleMax=7 \
  alpine:3.6 sleep 100000000000
```

Labels `com.df.scaleUpby=3` and `com.df.scaleDownby=2` configures how many replicas to scale up and down by respectively. Labels `com.df.scaleMin=1` and `com.df.scaleMax=7` denote the mininum and maximum number of replicas. We manually scale up this service by sending a POST request:

```bash
curl -X POST http://$(docker-machine ip swarm-1)/scaler/v1/scale-service -d \
'{"groupLabels": {"scale": "up", "service": "demo"}}'
```

We confirm that the `demo` service have been scaled up by 3 from 1 replica to 4 replicas:

```bash
docker service ls -f name=demo
```

Similarily, we manually scale down the service by sending:

```bash
curl -X POST http://$(docker-machine ip swarm-1)/scaler/v1/scale-service -d \
'{"groupLabels": {"scale": "down", "service": "demo"}}'
```

We confirm that the `demo` service has been scaled down by 2 from 4 replicas to 2 replicas:

```bash
docker service ls -f name=demo
```

Before we continuing, remove the `demo` service:

```bash
docker service rm demo
```

## Deploying Instrumented Service

The [go-demo](https://github.com/vfarcic/go-demo) service already exposes response time metrics with labels for *Docker Flow Monitor* to scrape. We deploy the service to be scaled based on the response time metrics:

```bash
docker stack deploy \
    -c stacks/go-demo-instrument-alert-short.yml \
    go-demo
```

The full stack definition can be found at [go-demo-instrument-alert-short.yml](https://github.com/thomasjpfan/docker-scaler/blob/master/stacks/go-demo-instrument-alert-short.yml). We will focus on the service labels for the `go-demo_main` service relating to scaling and alerting:

```yaml
main:
  ...
  deploy:
    ...
    labels:
      ...
      - com.df.alertName.1=resptimeabove
      - com.df.alertIf.1=@resp_time_above:0.1,5m,0.99
      - com.df.alertName.2=resptimebelow_unless_resptimeabove
      - com.df.alertIf.2=@resp_time_below:0.025,5m,0.75_unless_@resp_time_above:0.1,5m,0.9
      - com.df.alertLabels.2=receiver=system,service=go-demo_main,scale=down,type=service
      - com.df.alertAnnotations.2=summary=Response time of service go-demo_main is below 0.025 and not above 0.1
    ...
```

The `alertName`, `alertIf` and `alertFor` labels uses the [AlertIf Parameter Shortcuts](http://monitor.dockerflow.com/usage/#alertif-parameter-shortcuts) for creating Prometheus expressions that firing alerts. The alert `com.df.alertIf.1=@resp_time_above:0.1,5m,0.99` fires when the number of responses above 0.1 seconds in the last 5 minutes consist of more than 99% of all responses. The second alert fires when the number of response below 0.025 seconds in the last 5 minutes consist of more than 75% unless alert 1 is firing.

We can view the alerts generated by these labels:

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

Let's confirm that the go-demo stack is up-and-running:

```bash
docker stack ps -f desired-state=running go-demo
```

There should be three replicas of the `go-demo_main` service and one replica of the `go-demo_db` service. Please wait for all replicas to be up and running.

We can confirm that *Docker Flow Monitor* is monitoring the `go-demo` replicas:

```bash
open "http://$(docker-machine ip swarm-1)/monitor/targets"
```

There should be two or three targets depending on whether Prometheus already sent the alert to de-scale the service.

## Automatically Scaling Services

Let's go back to the Prometheus' alert screen:

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

By this time, the `godemo_main_resptimebelow_unless_resptimeabove` alert should be red due to having no requests sent to `go-demo`. The Alertmanager recieves this alert and sends a `POST` request to the `scaler` service to scale down `go-demo`. The label `com.df.scaleDownBy` on `go-demo_main` is set to 1 thus the number of replicas goes from 4 to 3.

Let's look at the logs of `scaler`:

```bash
docker service logs scaler_scaler
```

There should be a log message that states **Scaling go-demo_main from 4 to 3 replicas (min: 2)**. We can check that this happened:

```bash
docker service ls -f name=go-demo_main
```

The output should be similar to the following:

```bash
NAME                MODE                REPLICAS            IMAGE                    PORTS
go-demo_main        replicated          3/3                 vfarcic/go-demo:latest
```

Please visit your channel and you should see a Slack notification stating that `go-demo_main` has scaled from 4 to 3 replicas.

Let's see what happens when response times of the service becomes high by sending requests that will result in high response times:

```bash
for i in {1..30}; do
    DELAY=$[ $RANDOM % 6000 ]
    curl "http://$(docker-machine ip swarm-1)/demo/hello?delay=$DELAY"
done
```

Let's look at the alerts:

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

The `godemo_main_resptimeabove` turned red indicating that the threshold is reached. *Alertmanager* receives the alert, sends a `POST` request to the `scaler` service, and `docker-scaler` scales `go-demo_main` up by the value of `com.df.scaleUpBy`. In this case, the value of `com.df.scaleUpBy` is two. Let's look at the logs of `docker-scaler`:

```bash
docker service logs scaler_scaler
```

There should be a log message that states **Scaling go-demo_main from 3 to 5 replicas (max: 7)**. This message is also sent through Slack to notify us of this scaling event.

We can confirm that the number of replicas indeed scaled to three by querying the stack processes:

```bash
docker service ls -f name=go-demo_main
```

The output should look similar to the following:

```
NAME                MODE                REPLICAS            IMAGE                    PORTS
go-demo_main        replicated          5/5                 vfarcic/go-demo:latest
```

## What Now?

We just went through a simple example of a system that automatically scales and de-scales services. Feel free to add additional metrics and services to this self-adapting system to customize it to your needs.

Please remove the demo cluster we created and free your resources:

```bash
docker-machine rm -f swarm-1 swarm-2 swarm-3
```
