# Usage

*Docker Scaler* is controlled by sending HTTP requests to **[SCALER_IP]:[SCALER_PORT]**

## Scaling Services

This request queries docker service labels: `com.df.scaleMin`, `com.df.scaleMax`, `com.df.scaleDownBy`, `com.df.scaleUpBy` to determine how much to scale the service. Please see [configuration](configuration.md) for details.

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

## Scaling Nodes

- **URL:**
    `/v1/scale-nodes`

- **Method:**
    `POST`

- **Query Parameters:**

| Query | Description                                | Required |
|-------|--------------------------------------------|----------|
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
