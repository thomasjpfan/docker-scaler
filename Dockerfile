FROM golang:1.10.0-alpine3.7 as build
WORKDIR /go/src/github.com/thomasjpfan/docker-scaler
COPY . .
RUN go build -o docker-scaler main.go

FROM alpine:3.7
RUN apk add --no-cache tini ca-certificates

COPY --from=build /go/src/github.com/thomasjpfan/docker-scaler/docker-scaler /usr/local/bin/docker-scaler
RUN chmod +x /usr/local/bin/docker-scaler

ENV SERVER_PREFIX="/" \
    MIN_SCALE_LABEL="com.df.scaleMin" \
    MAX_SCALE_LABEL="com.df.scaleMax" \
    SCALE_DOWN_BY_LABEL="com.df.scaleDownBy" \
    SCALE_UP_BY_LABEL="com.df.scaleUpBy" \
    ALERT_SCALE_MIN="false" \
    ALERT_SCALE_MAX="true" \
    DEFAULT_MIN_REPLICAS="1" \
    DEFAULT_MAX_REPLICAS="5" \
    DEFAULT_SCALE_SERVICE_DOWN_BY="1" \
    DEFAULT_SCALE_SERVICE_UP_BY="1" \
    ALERTMANAGER_ADDRESS="http://alertmanager:9093" \
    ALERT_TIMEOUT="10" \
    RESCHEDULE_FILTER_LABEL="com.df.reschedule=true" \
    RESCHEDULE_TICKER_INTERVAL="60" \
    RESCHEDULE_TIMEOUT="1000" \
    RESCHEDULE_ENV_KEY="RESCHEDULE_DATE" \
    NODE_SCALER_BACKEND="none" \
    ALERT_NODE_MIN="false" \
    ALERT_NODE_MAX="true" \
    DEFAULT_MIN_MANAGER_NODES="3" \
    DEFAULT_MAX_MANAGER_NODES="7" \
    DEFAULT_MIN_WORKER_NODES="0" \
    DEFAULT_MAX_WORKER_NODES="5" \
    AWS_ENV_FILE="/run/secrets/aws" \
    AWS_DEFAULT_REGION="us-east-1"

EXPOSE 8080

ENTRYPOINT  ["/sbin/tini", "--"]
CMD ["docker-scaler"]
