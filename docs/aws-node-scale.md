# Auto-Scaling Nodes On Amazon Web Services

*Docker Scaler* includes endpoints to scale nodes on AWS. In this tutorial, we will construct a system that will scale up worker nodes based on memory usage. This tutorial uses [AWS CLI](https://aws.amazon.com/cli/) to communicate with AWS and [jq](https://stedolan.github.io/jq/download/) to parse json responses from the CLI.

!!! info
	If you are a Windows user, please run all the examples from *Git Bash* (installed through *Docker for Windows*). Also, make sure that your Git client is configured to check out the code *AS-IS*. Otherwise, Windows might change carriage returns to the Windows format.

## Setting up Current Environment

We will be using *Slack* webhooks to notify us. First, we create a *Slack* workspace and setup a webhook by consulting *Slack's* [Incoming Webhook](https://get.slack.help/hc/en-us/articles/115005265063-Incoming-WebHooks-for-Slack) page. After obtaining a webhook URL set it as an environment variable:

```bash
export SLACK_WEBHOOK_URL=[...]
```

The AWS CLI is configured by setting the following environment variables:

```bash
export AWS_ACCESS_KEY_ID=[...]
export AWS_SECRET_ACCESS_KEY=[...]
export AWS_DEFAULT_REGION=us-east-1
```

The *IAM Policies* required for this tutorial are `cloudformation:*`, `sqs:*`, `iam:*`, `ec2:*`, `lambda:*`, `dynamodb:*`, `"autoscaling:*`, and `elasticfilesystem:*`.

For convenience, we define the `STACK_NAME` to be the name of our AWS stack, `KEY_FILE` to be the path to the ssh AWS identity file, and `KEY_NAME` as the key's name on AWS.

```bash
export STACK_NAME=devops22
export KEY_FILE=devops22.pem # Location of pem file
export KEY_NAME=devops22
```

## Setting Up An AWS Cluster

Using AWS Cloudformation, we will create a cluster of three master ndoes:

```bash
aws cloudformation create-stack \
    --template-url https://editions-us-east-1.s3.amazonaws.com/aws/stable/Docker.tmpl \
    --capabilities CAPABILITY_IAM \
    --stack-name $STACK_NAME \
    --parameters \
    ParameterKey=ManagerSize,ParameterValue=3 \
    ParameterKey=ClusterSize,ParameterValue=0 \
    ParameterKey=KeyName,ParameterValue=$KEY_NAME \
    ParameterKey=EnableSystemPrune,ParameterValue=yes \
    ParameterKey=EnableCloudWatchLogs,ParameterValue=no \
    ParameterKey=EnableCloudStorEfs,ParameterValue=yes \
    ParameterKey=ManagerInstanceType,ParameterValue=t2.micro \
    ParameterKey=InstanceType,ParameterValue=t2.micro
```

We can check if the cluster came online by running:

```bash
aws cloudformation describe-stacks \
    --stack-name $STACK_NAME | \
    jq -r ".Stacks[0].StackStatus"
```

Please wait till the output of this command is `CREATE_COMPLETE` before continuing.

## Setting up the AWS Environment

 We need to log into a manager node to issue Docker commands and interact with our Docker swarm. To setup the manager shell environmental, we will define variables in our current shell and transfer them to the manager node.

We save the cluster dns to an environment variable `CLUSTER_DNS`:

```bash
CLUSTER_DNS=$(aws cloudformation \
    describe-stacks \
    --stack-name $STACK_NAME | \
    jq -r ".Stacks[0].Outputs[] | \
    select(.OutputKey==\"DefaultDNSTarget\")\
    .OutputValue")
```

We set the environment variable `CLUSTER_IP` to the public ip of one of the manager nodes:

```bash
CLUSTER_IP=$(aws ec2 describe-instances \
    | jq -r ".Reservations[] \
    .Instances[] \
    | select(.SecurityGroups[].GroupName \
    | contains(\"$STACK_NAME-ManagerVpcSG\"))\
    .PublicIpAddress" \
    | tail -n 1)
```

We save the the manager and worker autoscaling group names:

```bash
WORKER_ASG=$(aws autoscaling \
    describe-auto-scaling-groups \
    | jq -r ".AutoScalingGroups[] \
    | select(.AutoScalingGroupName \
    | startswith(\"$STACK_NAME-NodeAsg-\"))\
    .AutoScalingGroupName")

MANAGER_ASG=$(aws autoscaling \
    describe-auto-scaling-groups \
    | jq -r ".AutoScalingGroups[] \
    | select(.AutoScalingGroupName \
    | startswith(\"$STACK_NAME-ManagerAsg-\"))\
    .AutoScalingGroupName")
```

We clone the *Docker Scaler* repo and transfer the stacks folder

```bash
git clone https://github.com/thomasjpfan/docker-scaler.git

scp -i $KEY_FILE -rp docker-scaler/stacks docker@$CLUSTER_IP:~
```

Using ssh, we can transfer the environment variables into a file on the manager node:

```bash
echo "
export CLUSTER_DNS=$CLUSTER_DNS
export AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY
export AWS_DEFAULT_REGION=$AWS_DEFAULT_REGION
export WORKER_ASG=$WORKER_ASG
export MANAGER_ASG=$MANAGER_ASG
export SLACK_WEBHOOK_URL=$SLACK_WEBHOOK_URL
" | ssh -i $KEY_FILE docker@$CLUSTER_IP "cat > env"
```

Finally, we can log into the manager node and source the environment variables:

```bash
ssh -i $KEY_FILE docker@$CLUSTER_IP

source env
```

## Deploying Docker Flow Proxy (DFP) and Docker Flow Swarm Listener (DFSL)

For convenience, we will use *Docker Flow Proxy* and *Docker Flow Swarm Listener* to get a single access point to the cluster.

```bash
echo "admin:admin" | docker secret \
    create dfp_users_admin -

docker network create -d overlay proxy

docker stack deploy \
    -c stacks/docker-flow-proxy-aws.yml \
    proxy
```

Please visit [proxy.dockerflow.com](http://proxy.dockerflow.com) and [swarmlistener.dockerflow.com](http://swarmlistener.dockerflow.com/) for details on the *Docker Flow* stack.

## Deploying Docker Scaler

To allow *Docker Scaler* to access AWS, the credientials are stored in a Docker secret:

```bash
echo "
export AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY
" | docker secret create aws -
```

We can now deploy the *Docker Scaler* stack:

```bash
docker network create -d overlay scaler

docker stack deploy \
    -c stacks/docker-scaler-aws-tutorial.yml \
    scaler
```

This stack defines a single *Docker Scaler* service. Focusing on the environment variables set by the compose file:

```yaml
...
  services:
    scaler:
      image: thomasjpfan/scaler
      environment:
        - ALERTMANAGER_ADDRESS=http://alert-manager:9093
        - NODE_SCALER_BACKEND=aws
        - AWS_MANAGER_ASG=${MANAGER_ASG}
        - AWS_WORKER_ASG=${WORKER_ASG}
        - AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION}
        - SERVER_PREFIX=/scaler
      labels:
        - com.df.notify=true
        - com.df.distribute=true
        - com.df.servicePath=/scaler
        - com.df.port=8080
      secrets:
        - aws
...
```

The `NODE_SCALER_BACKEND` must be set to `aws` to configure *Docker Scaler* to scale nodes on AWS. The label `com.df.servicePath=/scaler` and environment variable `SERVER_PREFIX` opens up the `scaler` service to public REST calls. For this tutorial, we open this path to explore manually scaling nodes.

## Deploying Docker Flow Monitor and Alertmanager

The next stack defines the *Docker Flow Monitor* and *Alertmanager* services. Before we deploy the stack, we defined our *Alertmanager* configuration as a Docker secret:

```bash
echo "global:
  slack_api_url: '$SLACK_WEBHOOK_URL'
route:
  group_interval: 30m
  repeat_interval: 30m
  receiver: 'slack'
  group_by: [service, scale, type]
  routes:
    - match_re:
        scale: up|down
        type: node
      receiver: 'scale-nodes'
    - match_re:
        alertname: scale_service|reschedule_service|scale_nodes
      group_by: [alertname, service]
      group_wait: 5s
      group_interval: 15s
      receiver: 'slack-scaler'

receivers:
  - name: 'slack'
    slack_configs:
      - send_resolved: true
        title: '[{{ .Status | toUpper }}] {{ .GroupLabels.service }} service is in danger!'
        title_link: 'http://$CLUSTER_DNS/monitor/alerts'
        text: '{{ .CommonAnnotations.summary }}'
  - name: 'slack-scaler'
    slack_configs:
      - title: '{{ .GroupLabels.alertname }}: {{ .CommonAnnotations.request }}'
        color: '{{ if eq .CommonLabels.status \"error\" }}danger{{ else }}good{{ end }}'
        title_link: 'http://$CLUSTER_DNS/monitor/alerts'
        text: '{{ .CommonAnnotations.summary }}'
  - name: 'scale-nodes'
    webhook_configs:
      - send_resolved: false
        url: 'http://scaler:8080/v1/scale-nodes?by=1&type=worker'
" | docker secret create alert_manager_config -
```

This configuration groups alerts by their `service`, `scale`, and `type` labels. The `routes` section defines a `match_re` entry, that directs scale alerts to the `scale-nodes` reciever. The second route is configured to direct alerts from the `scaler` service to the `slack-scaler` receiver. The `scale-nodes` receivers `url` is given parameters `by=1` to denote how many nodes to scale down or up by, and `type=worker` to only scale worker nodes.

```bash
docker network create -d overlay monitor

DOMAIN=$CLUSTER_DNS \
    docker stack deploy \
    -c stacks/docker-flow-monitor-aws.yml \
    monitor
```

Let us confirm that the `monitor` stack is up and running:

```bash
docker stack ps monitor
```

Please wait a few moments for all the replicas to have the status `running`. After the `monitor` stack is up and running, we can test out manual node scaling!

## Manually Scaling Nodes

Before node scaling, we will deploy a simple sleeping service:

```bash
docker service create -d --replicas 6 \
  -l com.df.reschedule=true \
  --name demo \
  alpine:3.6 sleep 100000000000
```

The `com.df.reschedule=true` label signals to *Docker Scaler* that this service is allowed for rescheduling after node scaling.

The original cluster started out with three manager nodes. We now scale up the worker nodes by one be issuing a `POST` request:

```bash
curl -X POST http://$CLUSTER_DNS/scaler/v1/scale-nodes\?by\=1\&type\=worker -d \
'{"groupLabels": {"scale": "up"}}'
```

The parameters `by=1` and `type=worker` tell the service to scale worker nodes up by `1`. Inside the json request body, the `scale` value is set to `up` to denote scaling up. To scale nodes down just set the value to `down`. We can check the number of nodes by running:

```bash
docker node ls
```

The output should be similar to the following (node ids are discarded):

```
HOSTNAME                        STATUS              AVAILABILITY        MANAGER STATUS
ip-172-31-4-44.ec2.internal     Ready               Active              Reachable
ip-172-31-17-200.ec2.internal   Ready               Active              Reachable
ip-172-31-20-95.ec2.internal    Ready               Active
ip-172-31-44-49.ec2.internal    Ready               Active              Leader
```

If there are still three nodes, wait a few more minutes and try the command again. *Docker Scaler* waits for the new node to come up and reschedules services that are labeled `com.df.reschedule=true`. We look at the processes running on the new worker node:

```bash
docker node ps $(docker node ls -f role=worker -q)
```

The output should include some instances of the `demo` service, showing that the some of the instances has been place on the new node. We will now move on to implementing a system for automatic scaling!

## Deploying Node Exporters

The node exporters are used to display metrics about each nodes for *Docker Flow Monitor* to scrap. To deploy the exporters stack run:

```bash
docker stack deploy \
  -c stacks/exporters-aws.yml \
  exporter
```

We will focus on the service labels for the `node-exporter-manager` and `node-exporter-worker` services:

```yaml
...
services:
  ...
  node-exporter-manager:
    ...
    deploy:
      labels:
        ...
        - com.df.alertName.1=node_mem_limit_total_above
        - com.df.alertIf.1=@node_mem_limit_total_above:0.95
        - com.df.alertLabels.1=receiver=system,scale=no,service=exporter_node-exporter-manager,type=node
        - com.df.alertFor.1=30s
        ...
  node-exporter-worker:
    ...
    deploy:
      labels:
        ...
        - com.df.alertName.1=node_mem_limit_total_above
        - com.df.alertIf.1=@node_mem_limit_total_above:0.95
        - com.df.alertFor.1=30s
        - com.df.alertName.2=node_mem_limit_total_below
        - com.df.alertIf.2=@node_mem_limit_total_below:0.05
        - com.df.alertFor.2=30s
...
```

These labels use [AlertIf Parameter Shortcuts](http://monitor.dockerflow.com/usage/#alertif-parameter-shortcuts) for creating Prometheus expressions that firing alerts.

The `node-exporter-manager` has an `alertIf` label of `node_mem_limit_total_above:0.95`, which will fire when the total fractional memory of the all manager nodes is above 95%. Setting one of the `alertLabels` to `scale=no` prevents autoscaling and sends a notification to Slack. The `node-exproter-worker` has an `alertIf` label of `node_mem_limit_total_above:0.95` which will fire when the total fractional memory of all worker nodes is above 95%. Similiary, the `node_mem_limit_total_below:0.01` fires when the total fractional memory is below 5%. These values for memory alerts are extreme to prevent the alerts from firing. We will change these labels to explore what happens when they fire.

For example, we trigger alert 1 on `node-exporter-manager` by changing its alert label:

```bash
docker service update -d \
  --label-add "com.df.alertIf.1=@node_mem_limit_total_above:0.05" \
  exporter_node-exporter-manager
```

After the alert is fired, we can see a *Slack* notification stating **Total memory of the nodes is over 0.05**. Before we continue, we return the alert back to before:

```bash
docker service update -d \
  --label-add "com.df.alertIf.1=@node_mem_limit_total_above:0.95" \
  exporter_node-exporter-manager
```

## Automaticall Scaling Nodes

We trigger alert 1 on `node-exporter-worker` by setting the `node_mem_limit_total_above` limit to 5%:

```bash
docker service update -d \
  --label-add "com.df.alertIf.1=@node_mem_limit_total_above:0.05" \
  exporter_node-exporter-worker
```

After a few minutes, the alert will fire and trigger `scaler` to scale worker nodes up by one. *Slack* will send a notification stating: **Changing the number of worker nodes on aws from 1 to 2**. We confirm that there is now five nodes by running:

```bash
docker node ls
```

After the node comes up, `scaler` will also reschedule services with label, `com.df.reschedule=true`. During this process, *Slack* notifications were sent to inform us of each step. Before triggering the alert 2, we return alert 1 back to before:

```bash
docker service update -d \
  --label-add "com.df.alertIf.1=@node_mem_limit_total_above:0.95" \
  exporter_node-exporter-worker
```

We trigger the condition for scaling down a node by setting `node_mem_limit_total_below` limit to 95%:

```bash
docker service update -d \
  --label-add "com.df.alertIf.2=@node_mem_limit_total_below:0.95" \
  exporter_node-exporter-worker
```

*Slack* will send a notification stating: **Changing the number of worker nodes on aws from 2 to 1**. After a few minutes, the alert will fire and trigger `scaler` to scale worker nodes down by one. We confirm that there is now four nodes by running:

```bash
docker node ls
```

## What Now?

We just went through a simple example of a system that automatically scales and de-scales nodes. Feel free to add additional metrics and services to this self-adapting system to customize it to your needs.

Please remove the AWS cluster we created and free your resources:

```
 aws cloudformation delete-stack \
    --stack-name $STACK_NAME
```

You can navigate to [AWS Cloudformation](https://console.aws.amazon.com/cloudformation/home) to confirm that your stack has been removed.
