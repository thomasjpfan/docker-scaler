# Configuring Docker Scaler

*Docker Scaler* can be configured through Docker enivonment variables and/or by creating a new image based on `thomasjpfan/docker-scaler`

## Service Scaling Environment Variables

!!! tip
    The *Docker Scaler* container can be configured through envionment variables

The following environment variables can be used to configure the *Docker Scaler* relating to service scaling.

|Variable           |Description                                               |
|-------------------|----------------------------------------------------------|
| MIN_SCALE_LABEL   | Service label key with value representing the minimum number of replicas.<br>**Default:** `com.df.scaleMin` |
| MAX_SCALE_LABEL   | Service label key with value representing the maximum number of replicas.<br>**Default:** `com.df.scaleMax` |
| SCALE_DOWN_BY_LABEL | Service label key with value representing the number of replicas to scale down by.<br>**Default:** `com.df.scaleDownBy` |
| SCALE_UP_BY_LABEL | Service label key with value representing the number of replicas to scale up by.<br>**Default:** `com.df.scaleUpBy` |
| DEFAULT_MIN_REPLICAS | Default minimum number of replicas for a service.<br>**Default:** 1 |
| DEFAULT_MAX_REPLICAS | Default maximum number of replicas for a service.<br>**Default:** 5 |
| DEFAULT_SCALE_SERVICE_DOWN_BY | Default number of replicas to scale service down by.<br>**Default:** 1 |
| DEFAULT_SCALE_SERVICE_UP_BY | Default number of replicas to scale service up by.<br>**Default:** 1 |
| ALERTMANAGER_ADDRESS | Address for alertmanager.<br>**Default:** `http://alertmanager:9093` |
| ALERT_TIMEOUT | Alert timeout duration (seconds).<br>**Default:** 15 |
| RESCHEDULE_FILTER_LABEL | Services with this label will be rescheduled after node scaling.<br>**Default:** `com.df.reschedule=true"`|
| RESCHEDULE_TICKER_INTERVAL | Duration to wait when checking for nodes to come up (seconds).<br>**Default:** 20|
| RESCHEDULE_TIMEOUT | Time to wait for nodes to come up during rescheduling (seconds).<br>**Default:** 1000|
| RESCHEDULE_ENV_KEY | Key for env variable when rescheduling services.<br>**Default:** `RESCHEDULE_DATE`|

## Node Scaling Environment Variables

The following environment variables can be used to configure the *Docker Scaler* relating to node scaling.

| NODE_SCALER_BACKEND | Backend of node backend.<br>**Accepted Values:** [none, aws]<br>**Default:** none |
| DEFAULT_MIN_MANAGER_NODES | Miniumum number of manager nodes.<br>**Default:** 3 |
| DEFAULT_MAX_MANAGER_NODES | Maximum number of manager nodes.<br>**Default:** 7 |
| DEFAULT_MIN_WORKER_NODES | Miniumum number of worker nodes.<br>**Default:** 1 |
| DEFAULT_MAX_WORKER_NODES | Maximum number of worker nodes.<br>**Default:** 5 |

### AWS Node Scaling Envronment Variables

The following environment variables can be used to configure the *Docker Scaler* relating to AWS node scaling.

| AWS_ENV_FILE | Location of AWS env file used when `NODE_SCALER_BACKEND` is sent to `aws`.<br>**Default:** `/run/secrets/aws` |
| AWS_DEFAULT_REGION | Default AWS region.<br>**Default:** `us-east-1` |
| AWS_MANAGER_GROUP_NAME | AWS group name for manager nodes. |
| AWS_WORKER_GROUP_NAME | AWS group name for worker nodes.

#### AWS Secrets file

AWS secret file defines the necessary environment variables to authenticate with AWS.

```bash
echo 'export AWS_ACCESS_KEY_ID=xxxx
export AWS_SECRET_ACCESS_KEY=xxxx
' | docker secret create aws -
```