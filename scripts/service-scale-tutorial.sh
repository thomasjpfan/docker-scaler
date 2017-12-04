# Setting Up A Cluster
git clone https://github.com/thomasjpfan/docker-scaler.git

cd docker-scaler

./scripts/ds-swarm.sh

eval $(docker-machine env swarm-1)

# Deploying Docker Flow Proxy (DFP) and Docker Flow Swarm Listener (DFSL)

docker network create -d overlay proxy

docker stack deploy \
    -c stacks/docker-flow-proxy-mem.yml \
    proxy

# Deploying Docker Scaler

docker network create -d overlay scaler

docker stack deploy \
    -c stacks/docker-scaler-basic-tutorial.yml \
    scaler

## Deploying Docker Flow Monitor and Alertmanager

echo "global:
  slack_api_url: 'https://hooks.slack.com/services/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'
route:
  group_interval: 10s
  repeat_interval: 30s
  group_wait: 5s
  receiver: 'slack'
  routes:
  - match_re:
      scale: up|down
      type: service
    receiver: 'scale'
    group_by: [service, scale, type]
  - match_re:
      alertname: scale_service|reschedule_service|scale_nodes
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

docker network create -d overlay monitor

DOMAIN=$(docker-machine ip swarm-1) \
    docker stack deploy \
    -c stacks/docker-flow-monitor-slack.yml \
    monitor

docker stack ps monitor

## Deploying Instrumented Service

docker stack deploy \
    -c stacks/go-demo-instrument-alert-short.yml \
    go-demo

open "http://$(docker-machine ip swarm-1)/monitor/alerts"

docker stack ps -f desired-state=running go-demo

open "http://$(docker-machine ip swarm-1)/monitor/targets"

## Automatically Scaling Services

open "http://$(docker-machine ip swarm-1)/monitor/alerts"

docker service logs scaler_scaler

docker service ls -f name=go-demo_main

for i in {1..30}; do
    DELAY=$[ $RANDOM % 6000 ]
    curl "http://$(docker-machine ip swarm-1)/demo/hello?delay=$DELAY"
done

open "http://$(docker-machine ip swarm-1)/monitor/alerts"

docker service logs scaler_docker-scaler

docker service ls -f name=go-demo_main

docker-machine rm -f swarm-1 swarm-2 swarm-3
