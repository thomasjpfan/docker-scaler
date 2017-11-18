# Auto-Scaling With Docker Scaler And Instrumented Metrics

*Docker Scaler* provides an alternative to using Jenkins for service scaling shown in Docker Flow Monitor's [auto-scaling tutorial](http://monitor.dockerflow.com/auto-scaling/). In this tutorial, we will construct a system that will scale a service based on response time. The following is an overview of the triggered events in our self-adapting system:

1. The [go-demo](https://github.com/vfarcic/go-demo) service response times becomes too high.
2. [Docker Flow Monitor](http://monitor.dockerflow.com/) is querying the services' metrics, notices the high response times, and alerts the [Alertmanager](https://prometheus.io/docs/alerting/alertmanager/).
3. The Alertmanager is configured to forward the alert to *Docker Scaler*.
4. *Docker Scaler* scales the service up.

This tutorial assumes you have Docker Machine version v0.8+ that includes Docker Engine v1.12+.

!!! info
	If you are a Windows user, please run all the examples from *Git Bash* (installed through *Docker for Windows*). Also, make sure that your Git client is configured to check out the code *AS-IS*. Otherwise, Windows might change carriage returns to the Windows format.

## Setting Up A Cluster

!!! info
    Feel free to skip this section if you already have a Swarm cluster that can be used for this tutorial

We'll create a Swarm cluster consisting of three nodes created with Docker Machine.

```bash
git clone https://github.com/thomasjpfan/docker-scaler.git

cd docker-scaler

./scripts/ds-swarm.sh

eval $(docker-machine env swarm-1)
```

We cloned the [thomasjpfan/docker-scaler](https://github.com/thomasjpfan/docker-scaler) respository. It contains all the scripts and stack files we will use throughout this tutorial. Next, we executed the `ds-swarm.sh` script that created the cluster. Finally, we used the `eval` command to tell our local Docker client to use the remote Docker Engine `swarm-1`.

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
    -c stacks/docker-scaler-basic-tutorial.yml \
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
      volumes:
        - /var/run/docker.sock:/var/run/docker.sock
      networks:
        - scaler
      deploy:
        placement:
            constraints: [node.role == manager]
...
```

This definition constraints *Docker Scaler* to run on manager nodes and gives it access to the Docker socket, so that it can scale services in the cluster.

## Deploying Docker Flow Monitor and Alertmanager

The next stack defines the *Docker Flow Monitor* and *Alertmanager* services. Before we deploy the stack, we defined our *Alertmanager* configuration as a Docker secret:

```bash
echo "global:
  slack_api_url: 'https://hooks.slack.com/services/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'
route:
  group_by: [service, scale, type]
  repeat_interval: 5m
  group_interval: 5m
  receiver: 'slack'
  routes:
  - match_re:
      scale: ^(up|down)$
      type: ^service$
    receiver: 'scale'
  - match:
      alertname: scale_service
    group_by: [alertname, service]
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
      - title: 'Docker Scaler triggered {{ .GroupLabels.alertname }}'
        color: 'good'
        title_link: 'http://$(docker-machine ip swarm-1)/monitor/alerts'
        text: '{{ .CommonAnnotations.summary }}'
  - name: 'scale'
    webhook_configs:
      - send_resolved: false
        url: 'http://scaler:8080/v1/scale-service'
" | docker secret create alert_manager_config -
```
This configuration groups alerts by their `service`, `scale`, and `type` labels. The `routes` section defines a `match_re` entry, that directs scale alerts to the `scale` reciever. Another route is configured to direct alerts from the `scaler` service to the `slack-scaler` receiver.

Now we can deploy the monitor `monitor` stack.

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
    image: prom/alertmanager
    networks:
      - monitor
      - scaler
    secrets:
      - alert_manager_config
    command: -config.file=/run/secrets/alert_manager_config -storage.path=/alertmanager
...
```

With access to the `scaler` network, `alert-manager` can send scaling requests to the `scaler` service. For information about the Docker Flow Monitor stack can be found in its [documentation](http://monitor.dockerflow.com).

Let us confirm that the `monitor` stack is up and running:

```bash
docker stack ps monitor
```

Please wait a few moments for all the replicas to have the status `running`. After the `monitor` stack is up and running, we can start deploying the `go-demo_main` service!

## Deploying Instrumented Service

The [go-demo](https://github.com/vfarcic/go-demo) service already exposes response time metrics with labels for *Docker Flow Monitor* to scrape. We can deploy the service to be scaled based on the response time metrics:

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
      - com.df.scaleMin=2
      - com.df.scaleMax=5
      - com.df.scaleDownBy=1
      - com.df.scaleUpBy=2
      - com.df.alertName.1=mem_limit
      - com.df.alertIf.1=@service_mem_limit:0.8
      - com.df.alertFor.1=5m
      - com.df.alertName.2=resp_time_above
      - com.df.alertIf.2=@resp_time_above:0.1,5m,0.99
      - com.df.alertName.3=resp_time_below
      - com.df.alertIf.3=@resp_time_below:0.025,5m,0.75
    ...
```
The `scaleMin` and `scaleMax` labels are used by *Docker Scaler* to bound the number replicas for the `go-main_main` service. The `alertName`, `alertIf` and `alertFor` labels uses the [AlertIf Parameter Shortcuts](http://monitor.dockerflow.com/usage/#alertif-parameter-shortcuts) for creating full Prometheus expressions that translate into alerts. We can view the alerts generated by these labels:

```bash
open "http://$(docker-machine ip swarm-1)/monitor/alerts"
```

Docker Flow Monitor translates the alert labeled `resp_time_above` into an alert called `godemo_main_resp_time_above` with the following definition:

```
ALERT godemo_main_resp_tim_eabove
  IF sum(rate(http_server_resp_time_bucket{job="go-demo_main",le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="go-demo_main"}[5m])) < 0.99
  LABELS {receiver="system", scale="up", service="go-demo_main"}
  ANNOTATIONS {summary="Response time of the service go-demo_main is above 0.1"}
```

This alert is triggered when the response times of the `0.1` seconds bucket is above 99% for over five minutes. Notice that the alert is labeled with `scale=up` to comminucate to the *Alertmanager* that the `go-demo_main` service should be scaled up.

Similiarly, the alert labeled `resp_time_below` is translated into an alert called `godemo_main_resp_time_below`. It is labeled with `scale=down` to trigger a de-scaling event:

```
ALERT godemo_main_resp_time_below
  IF sum(rate(http_server_resp_time_bucket{job="go-demo_main",le="0.025"}[5m])) / sum(rate(http_server_resp_time_count{job="go-demo_main"}[5m])) > 0.75
  LABELS {receiver="system", scale="down", service="go-demo_main"}
  ANNOTATIONS {summary="Response time of the service go-demo_main is below 0.025"}
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

By this time, the `godemo_main_resp_time_below` alert should be red since the `go-demo_main` service has a response faster than twenty-five milliseconds limit we set. The Alertmanager recieves this alert and sends a `POST` request to the `docker-scaler` service to scale down `go-demo`. The label `com.df.scaleDownBy` on `go-demo_main` is set to 1 thus the number of replicas goes from 3 to 2.

Let's look at the logs of `docker-scaler`:

```bash
docker service logs scaler_scaler
```

There should be a log message that states **Scaling go-demo_main from 3 to 2 replicas**. We can check that this happened:

```bash
docker service ls -f name=go-demo_main
```

The output should be similar to the following:

```
NAME                MODE                REPLICAS            IMAGE                    PORTS
go-demo_main        replicated          2/2                 vfarcic/go-demo:latest
```

Please visit the **#df-monitor-tests** channel inside [devops20.slack.com](https://devops20.slack.com/) and you should see a Slack notification stating that **go-demo_main could not be scaled**. If this is your first visit to **devops20** on Slack, you'll have to register through [slack.devops20toolkit.com](http://slack.devops20toolkit.com/).

Let's see what happens when response times of the service becomes too high by sending requests that will result in high response times:

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

The `godemo_main_resp_time_above` turned red indicating that the threshold is reached. *Alertmanager* receives the alert, sends a `POST` request to the `docker-scaler` service, and `docker-scaler` scales `go-demo_main` up by the value of `com.df.scaleUpBy`. In this case, the value of `com.df.scaleUpBy` is two. Let's look at the logs of `docker-scaler`:

```bash
docker service logs scaler_docker-scaler
```

There should be a log message that states **Scaling go-demo_main from 2 to 4 replicas**. This message is also sent through Slack to notify us of this scaling event.

We can confirm that the number of replicas indeed scaled to three by querying the stack processes:

```bash
docker service ls -f name=go-demo_main
```

The output should look similar to the following:

```
NAME                MODE                REPLICAS            IMAGE                    PORTS
go-demo_main        replicated          4/4                 vfarcic/go-demo:latest
```

## What Now?

You saw a simple example of a system that automatically scales and de-scales services. Feel free to add additional metrics and services to this self-adapting system to customize it to your needs.

Please remove the demo cluster we created and free your resources:

```bash
docker-machine rm -f swarm-1 swarm-2 swarm-3
```
