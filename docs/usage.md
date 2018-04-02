# Usage

*Docker Scaler* is controlled by sending HTTP requests to **[SCALER_IP]:[SCALER_PORT]**

## Scaling Services

### Alertmanager Webhook

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

### User Friendly Endpoint

The whole scaling event can be place in the url:

- **URL:**
    `/v1/reschedule-service`

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

The request body conforms to the alertmanager notifications so that *Docker Scaler* can be used as a webhook.

- **URL:**
    `/v1/scale-nodes`

- **Method:**
    `POST`

- **Query Parameters:**

| Query | Description                                | Required |
| ----- | ------------------------------------------ | -------- |
| by    | The number of nodes to scale up or down by | yes      |

- **Request Body:**

```json
{
    "groupLabels": {
        "scale": "down",
    }
}
```

The `scale` value accepts `up` for scaling up and `down` for scaling down.
