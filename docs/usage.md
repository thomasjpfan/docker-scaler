# Usage

*Docker Scaler* is controlled by sending HTTP requests to **[SCALER_IP]:[SCALER_PORT]**

## Scaling Services

### Scaling Services - Alertmanager Webhook

This request queries docker service labels: `com.df.scaleMin`, `com.df.scaleMax`, `com.df.scaleDownBy`, `com.df.scaleUpBy` to determine how much to scale the service by. The request body conforms to the alertmanager notifications so that *Docker Scaler* can be used as a webhook.

- **URL:**
    `/v1/scale-service`

- **Method:**
    `POST`

- **Request Body:**

```json
{
    "groupLabels": {
        "scale": "up",
        "service": "example_web"
    }
}
```

The `service` value is the name of the service to scale. The `scale` value accepts `up` for scaling up and `down` for scaling down.

| Service Label        | Description                         |
|----------------------|-------------------------------------|
| `com.df.scaleMin`    | Minimum number of replicas          |
| `com.df.scaleMax`    | Maximum number of replicas          |
| `com.df.scaleDownBy` | Number of replicas to scale down by |
| `com.df.scaleUpBy`   | Number of replicas to scale up by   |

### Scaling Services - User Friendly Endpoint

The whole scaling event can be place in the url:

- **URL:**
    `/v1/scale-service`

- **Method:**
    `POST`

- **Query Parameters:**

| Query   | Description                         | Required |
| ------- | ----------------------------------- | -------- |
| service | Name of service to reschedule       | yes      |
| scale   | Direction to scale (`up` or `down`) | yes      |
| by      | Number of replicas to scale by      | yes      |

The `com.df.scaleMax` and `com.df.scaleMin` will still be used to bound the number of replicas for the service.

## Rescheduling All Services

This request only reschedule services with label: `com.df.reschedule=true`. See [Configuration](configuration.md) to change this default.

- **URL:**
    `/v1/reschedule-services`

- **Method:**
    `POST`

## Rescheduling One Service

This request only reschedule target service with label: `com.df.reschedule=true`. See [Configuration](configuration.md) to change this default.

- **URL:**
    `/v1/reschedule-service`

- **Method:**
    `POST`

- **Query Parameters:**

| Query   | Description                   | Required |
| ------- | ----------------------------- | -------- |
| service | Name of service to reschedule | yes      |

## Scaling Nodes

The node scaling feature is activated by setting `NODE_SCALER_BACKEND` to a backend.

### Scaling Nodes - AlertManager Webhook

The request body conforms to the alertmanager notifications so that *Docker Scaler* can be used as a webhook.

- **URL:**
    `/v1/scale-nodes`

- **Method:**
    `POST`


- **Request Body:**

```json
{
    "groupLabels": {
        "scale": "down",
        "service": "node_exporter",
    }
}
```

The `scale` value accepts `up` for scaling up and `down` for scaling down. When *Docker Scaler* it setup to send the service name that trigger the alert, the `service` parameter can be passed on to *Docker Scaler* as a group label. *Docker Scaler* will use the following labels on the `service` to configure how to scale nodes:

| Service Label                   | Description                               |
|---------------------------------|-------------------------------------------|
| `com.df.scaleManagerNodeMin`    | Minimum number of managers                |
| `com.df.scaleManagerNodeMax`    | Maximum number of managers                |
| `com.df.scaleManagerNodeDownBy` | Number of nodes to scale managers down by |
| `com.df.scaleManagerNodeUpBy`   | Number of nodes to scale managers up by   |
| `com.df.scaleWorkerNodeMin`     | Minimum number of workers                 |
| `com.df.scaleWorkerNodeMax`     | Maximum number of workers                 |
| `com.df.scaleWorkerNodeDownBy`  | Number of nodes to scale workers down by  |
| `com.df.scaleWorkerNodeUpBy`    | Number of nodes to scale workers up by    |

### Scaling Nodes - User Friendly Endpoint

The whole node scaling event can be place in the url:

- **URL:**
    `/v1/scale-nodes`

- **Method:**
    `POST`

- **Query Parameters:**

| Query | Description                                   | Required |
| ----- | --------------------------------------------- | -------- |
| by    | The number of nodes to scale up or down by    | yes      |
| scale | Direction to scale (`up` or `down`)           | yes      |
| type  | Type of node to scale (`manager` or `worker`) | yes      |
